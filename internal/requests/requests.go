package requests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	requestmodel "github.com/FreekingDean/gojellyfin/internal/store/request"
	submissionmodel "github.com/FreekingDean/gojellyfin/internal/store/requestsubmission"
)

const tvdbProvider = "Tvdb"

var ErrNowhere = errors.New("no source takes requests for this kind of title")

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
	Mine      bool
}

func downloaderFor(title discover.Title) sources.Kind {
	if title.Kind == itemmodel.KindSeries {
		return sources.KindSonarr
	}

	return sources.KindRadarr
}

func (s *Service) Request(ctx context.Context, userID uuid.UUID, title discover.Title) error {
	targets, err := s.downloaders.Requestable(ctx, downloaderFor(title))
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return ErrNowhere
	}

	name := ""
	if title.Metadata.Name != nil {
		name = *title.Metadata.Name
	}

	err = s.store.Request.Create().
		SetUserID(userID).
		SetKey(title.Key).
		SetKind(requestmodel.Kind(title.Kind)).
		SetTmdbID(int(title.TmdbID)).
		SetName(name).
		OnConflictColumns(requestmodel.FieldUserID, requestmodel.FieldKey).
		DoNothing().
		Exec(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to record the request for %s: %w", title.Key, err)
	}

	wanted := sources.Wanted{Name: name, TmdbID: int(title.TmdbID), TvdbID: tvdbID(title)}
	for _, target := range targets {
		if err := s.submit(ctx, target, title.Key, wanted); err != nil {
			return err
		}
	}

	return nil
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
		Select(requestmodel.FieldKey, requestmodel.FieldUserID).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the requests: %w", err)
	}

	for _, record := range records {
		mark := marks[record.Key]
		mark.Requested = true
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
