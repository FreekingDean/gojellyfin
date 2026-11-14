package requests

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	requestmodel "github.com/FreekingDean/gojellyfin/internal/store/request"
	submissionmodel "github.com/FreekingDean/gojellyfin/internal/store/requestsubmission"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
)

const tvdbProvider = "Tvdb"

const QuotaWindow = 7 * 24 * time.Hour

var (
	ErrNowhere    = errors.New("no source takes requests for this kind of title")
	ErrNotAllowed = errors.New("this user may not make requests")
	ErrQuota      = errors.New("this user has asked for as much as a week allows")
	ErrNotFound   = errors.New("no such request")
)

type (
	Request    = store.RequestModel
	Submission = store.RequestSubmissionModel
)

type Downloaders interface {
	Requestable(ctx context.Context, kind sources.Kind) ([]sources.Source, error)
	Submit(ctx context.Context, source sources.Source, wanted sources.Wanted) error
}

type Service struct {
	store       *store.Client
	downloaders Downloaders
}

func New(store *store.Client, downloaders *sources.Service) *Service {
	return newService(store, downloaders)
}

func newService(store *store.Client, downloaders Downloaders) *Service {
	return &Service{store: store, downloaders: downloaders}
}

type Mark struct {
	Requested bool
	Pending   bool
	Mine      bool
}

func downloaderOf(kind items.Kind) sources.Kind {
	if kind == itemmodel.KindSeries {
		return sources.KindSonarr
	}

	return sources.KindRadarr
}

func (s *Service) Taken(ctx context.Context, kind items.Kind) (bool, error) {
	targets, err := s.downloaders.Requestable(ctx, downloaderOf(kind))
	if err != nil {
		return false, err
	}

	return len(targets) > 0, nil
}

func (s *Service) Request(ctx context.Context, userID uuid.UUID, title discover.Title) error {
	targets, err := s.downloaders.Requestable(ctx, downloaderOf(title.Kind))
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return ErrNowhere
	}

	asker, err := s.store.User.Query().Where(usermodel.ID(userID)).WithPolicy().OnlyModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to query who is asking: %w", err)
	}
	if !asker.CanRequest {
		return ErrNotAllowed
	}

	asked, err := s.asked(ctx, asker, title)
	if err != nil {
		return err
	}
	if asked.Status != requestmodel.StatusApproved {
		return nil
	}

	return s.submitAll(ctx, targets, asked)
}

func (s *Service) asked(ctx context.Context, asker *store.UserModel, title discover.Title) (*Request, error) {
	existing, err := s.store.Request.Query().
		Where(requestmodel.UserID(asker.ID), requestmodel.Key(title.Key)).
		OnlyModel(ctx)
	if err == nil {
		return existing, nil
	}
	if !store.IsNotFound(err) {
		return nil, fmt.Errorf("failed to query the request for %s: %w", title.Key, err)
	}

	if asker.RequestQuota != nil {
		used, err := s.used(ctx, asker.ID)
		if err != nil {
			return nil, err
		}
		if used >= *asker.RequestQuota {
			return nil, ErrQuota
		}
	}

	name := ""
	if title.Metadata.Name != nil {
		name = *title.Metadata.Name
	}

	status := requestmodel.StatusApproved
	if asker.RequestsNeedApproval && (asker.Policy == nil || !asker.Policy.IsAdministrator) {
		status = requestmodel.StatusPending
	}

	created, err := s.store.Request.Create().
		SetUserID(asker.ID).
		SetKey(title.Key).
		SetKind(requestmodel.Kind(title.Kind)).
		SetTmdbID(int(title.TmdbID)).
		SetTvdbID(tvdbID(title)).
		SetName(name).
		SetStatus(status).
		SaveModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to record the request for %s: %w", title.Key, err)
	}

	return created, nil
}

func (s *Service) used(ctx context.Context, userID uuid.UUID) (int, error) {
	used, err := s.store.Request.Query().
		Where(requestmodel.UserID(userID), requestmodel.CreatedAtGT(time.Now().Add(-QuotaWindow))).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count what was asked for lately: %w", err)
	}

	return used, nil
}

func (s *Service) submitAll(ctx context.Context, targets []sources.Source, asked *Request) error {
	wanted := sources.Wanted{Name: asked.Name, TmdbID: asked.TmdbID, TvdbID: asked.TvdbID}
	for _, target := range targets {
		if err := s.submit(ctx, target, asked.Key, wanted); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) Approve(ctx context.Context, requestID uuid.UUID) error {
	asked, err := s.decide(ctx, requestID, requestmodel.StatusApproved)
	if err != nil {
		return err
	}

	kind := sources.KindRadarr
	if asked.Kind == requestmodel.KindSeries {
		kind = sources.KindSonarr
	}

	targets, err := s.downloaders.Requestable(ctx, kind)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return ErrNowhere
	}

	return s.submitAll(ctx, targets, asked)
}

func (s *Service) Decline(ctx context.Context, requestID uuid.UUID) error {
	_, err := s.decide(ctx, requestID, requestmodel.StatusDeclined)

	return err
}

func (s *Service) decide(ctx context.Context, requestID uuid.UUID, status requestmodel.Status) (*Request, error) {
	decided, err := s.store.Request.UpdateOneID(requestID).SetStatus(status).SaveModel(ctx)
	if store.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to record the decision: %w", err)
	}

	return decided, nil
}

func tvdbID(title discover.Title) int {
	if title.Metadata.ProviderIds == nil {
		return 0
	}

	id, err := strconv.Atoi((*title.Metadata.ProviderIds)[tvdbProvider])
	if err != nil {
		return 0
	}

	return id
}

func (s *Service) submit(ctx context.Context, target sources.Source, key string, wanted sources.Wanted) error {
	accepted, err := s.store.RequestSubmission.Query().
		Where(submissionmodel.SourceID(target.ID), submissionmodel.Key(key), submissionmodel.AcceptedAtNotNil()).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to query what %s was asked for: %w", target.Name, err)
	}
	if accepted {
		return nil
	}

	submission := s.store.RequestSubmission.Create().SetSourceID(target.ID).SetKey(key)
	if refused := s.downloaders.Submit(ctx, target, wanted); refused != nil {
		log.Printf("requests: %s did not take %s: %v", target.Name, key, refused)
		submission.SetLastError(refused.Error())
	} else {
		submission.SetAcceptedAt(time.Now()).SetLastError("")
	}

	err = submission.
		OnConflictColumns(submissionmodel.FieldSourceID, submissionmodel.FieldKey).
		UpdateAcceptedAt().
		UpdateLastError().
		UpdateUpdatedAt().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to record what %s answered for %s: %w", target.Name, key, err)
	}

	return nil
}

func (s *Service) Withdraw(ctx context.Context, userID uuid.UUID, key string) error {
	_, err := s.store.Request.Delete().
		Where(requestmodel.UserID(userID), requestmodel.Key(key)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to withdraw the request for %s: %w", key, err)
	}

	return nil
}

func (s *Service) Marks(ctx context.Context, userID uuid.UUID, keys []string) (map[string]Mark, error) {
	marks := map[string]Mark{}
	if len(keys) == 0 {
		return marks, nil
	}

	records, err := s.store.Request.Query().
		Where(requestmodel.KeyIn(keys...)).
		Select(requestmodel.FieldKey, requestmodel.FieldUserID, requestmodel.FieldStatus).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the requests: %w", err)
	}

	for _, record := range records {
		if record.Status == requestmodel.StatusDeclined {
			continue
		}

		mark := marks[record.Key]
		mark.Requested = mark.Requested || record.Status == requestmodel.StatusApproved
		mark.Pending = mark.Pending || record.Status == requestmodel.StatusPending
		mark.Mine = mark.Mine || record.UserID == userID
		marks[record.Key] = mark
	}

	return marks, nil
}

func (s *Service) Submissions(ctx context.Context, key string) ([]*Submission, error) {
	records, err := s.store.RequestSubmission.Query().
		Where(submissionmodel.Key(key)).
		Order(submissionmodel.ByCreatedAt()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query what was asked for %s: %w", key, err)
	}

	return records, nil
}
