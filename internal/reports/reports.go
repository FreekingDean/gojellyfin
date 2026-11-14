package reports

import (
	"context"
	"errors"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	filemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	reportmodel "github.com/FreekingDean/gojellyfin/internal/store/report"
)

type (
	Report = store.ReportModel
	Kind   = reportmodel.Kind
	Origin = reportmodel.Origin
	Status = reportmodel.Status
)

const (
	KindUnplayable = reportmodel.KindUnplayable
	KindVideo      = reportmodel.KindVideo
	KindAudio      = reportmodel.KindAudio
	KindSubtitles  = reportmodel.KindSubtitles
	KindOther      = reportmodel.KindOther

	OriginUser     = reportmodel.OriginUser
	OriginDelete   = reportmodel.OriginDelete
	OriginProbe    = reportmodel.OriginProbe
	OriginPlayback = reportmodel.OriginPlayback

	StatusOpen      = reportmodel.StatusOpen
	StatusResolved  = reportmodel.StatusResolved
	StatusDismissed = reportmodel.StatusDismissed
)

var (
	ErrNotFound         = errors.New("no such report")
	ErrNothingToReplace = errors.New("no downloader names a file for this title")

	ValidKind = reportmodel.KindValidator
)

func Takes(kind itemmodel.Kind) bool {
	return kind == itemmodel.KindMovie || kind == itemmodel.KindEpisode
}

type Downloaders interface {
	Replace(ctx context.Context, sourceID uuid.UUID, broken sources.Copy) error
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

type Filing struct {
	ItemID uuid.UUID
	FileID *uuid.UUID
	UserID *uuid.UUID
	Kind   Kind
	Origin Origin
	Note   string
}

func (s *Service) File(ctx context.Context, filing Filing) error {
	standing := []Status{StatusOpen}
	if filing.UserID == nil {
		standing = append(standing, StatusDismissed)
	}

	same := s.store.Report.Query().Where(
		reportmodel.ItemID(filing.ItemID),
		reportmodel.KindEQ(filing.Kind),
		reportmodel.OriginEQ(filing.Origin),
		reportmodel.StatusIn(standing...),
	)
	if filing.FileID != nil {
		same = same.Where(reportmodel.ItemSourceID(*filing.FileID))
	} else {
		same = same.Where(reportmodel.ItemSourceIDIsNil())
	}
	if filing.UserID != nil {
		same = same.Where(reportmodel.UserID(*filing.UserID))
	} else {
		same = same.Where(reportmodel.UserIDIsNil())
	}

	filed, err := same.Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to query the open reports: %w", err)
	}
	if filed {
		return nil
	}

	err = s.store.Report.Create().
		SetItemID(filing.ItemID).
		SetNillableItemSourceID(filing.FileID).
		SetNillableUserID(filing.UserID).
		SetKind(filing.Kind).
		SetOrigin(filing.Origin).
		SetNote(filing.Note).
		Exec(ctx)
	if store.IsConstraintError(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to file the report: %w", err)
	}

	return nil
}

func (s *Service) Queue(ctx context.Context) ([]*Report, error) {
	filed, err := s.store.Report.Query().
		WithItem(func(reported *store.ItemQuery) {
			reported.WithItemSources().WithParent(func(season *store.ItemQuery) { season.WithParent() })
		}).
		WithFile().
		WithUser().
		Order(reportmodel.ByCreatedAt(sql.OrderDesc())).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the reports: %w", err)
	}

	return filed, nil
}

func Replaceable(filed *Report) bool {
	if filed.Item == nil {
		return false
	}

	for _, file := range filed.Item.ItemSources {
		named := file.ExternalID != nil && file.ExternalFileID != nil
		if named && (filed.ItemSourceID == nil || *filed.ItemSourceID == file.ID) {
			return true
		}
	}

	return false
}

func (s *Service) Resolve(ctx context.Context, reportID uuid.UUID) error {
	return s.decide(ctx, reportID, StatusResolved)
}

func (s *Service) Dismiss(ctx context.Context, reportID uuid.UUID) error {
	return s.decide(ctx, reportID, StatusDismissed)
}

func (s *Service) decide(ctx context.Context, reportID uuid.UUID, status Status) error {
	err := s.store.Report.UpdateOneID(reportID).SetStatus(status).Exec(ctx)
	if store.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to record the decision: %w", err)
	}

	return nil
}

func (s *Service) Replace(ctx context.Context, reportID uuid.UUID) error {
	filed, err := s.store.Report.Query().Where(reportmodel.ID(reportID)).OnlyModel(ctx)
	if store.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to query the report: %w", err)
	}

	named := s.store.ItemSource.Query().Where(
		filemodel.ItemID(filed.ItemID),
		filemodel.ExternalIDNotNil(),
		filemodel.ExternalFileIDNotNil(),
	)
	if filed.ItemSourceID != nil {
		named = named.Where(filemodel.ID(*filed.ItemSourceID))
	}

	files, err := named.AllModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to query the reported files: %w", err)
	}
	if len(files) == 0 {
		return ErrNothingToReplace
	}

	for _, file := range files {
		broken := sources.Copy{TitleID: *file.ExternalID, FileID: *file.ExternalFileID}
		if err := s.downloaders.Replace(ctx, file.SourceID, broken); err != nil {
			return fmt.Errorf("failed to replace %s: %w", file.Path, err)
		}
	}

	_, err = s.store.Report.Update().
		Where(reportmodel.ItemID(filed.ItemID), reportmodel.StatusEQ(StatusOpen)).
		SetStatus(StatusResolved).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve the reports: %w", err)
	}

	return nil
}
