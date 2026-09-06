package items

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const (
	RefreshItemJobID = "RefreshItem"

	ParamItem    = "item"
	ParamLibrary = "library"
	ParamSource  = "source"
)

type Scanned struct {
	Key         string
	ParentKey   string
	Tagged      bool
	Kind        Kind
	Name        string
	SortName    string
	Year        *int32
	Index       *int32
	ParentIndex *int32
	Files       []ScannedFile
}

type ScannedFile struct {
	Path         string
	DateModified time.Time
}

func (s *Service) RefreshItemJob() jobs.Job {
	return jobs.Job{
		Name:        RefreshItemJobID,
		Category:    "Library",
		Description: "Writes one title, its files and its library membership.",
		Run:         s.refreshItem,
	}
}

func (s *Service) refreshItem(ctx context.Context) error {
	scanned, err := jobs.GetParam[Scanned](ctx, ParamItem)
	if err != nil {
		return err
	}

	libraryID, err := jobs.GetParam[uuid.UUID](ctx, ParamLibrary)
	if err != nil {
		return err
	}

	sourceID, err := jobs.GetParam[uuid.UUID](ctx, ParamSource)
	if err != nil {
		return err
	}

	return s.RefreshItem(ctx, libraryID, sourceID, scanned)
}

func (s *Service) RefreshItem(ctx context.Context, libraryID, sourceID uuid.UUID, scanned Scanned) error {
	parentID, err := s.parentOf(ctx, scanned.ParentKey)
	if err != nil {
		return err
	}

	item, err := s.SaveScanned(ctx, Item{
		ParentID:          parentID,
		Kind:              scanned.Kind,
		Key:               scanned.Key,
		Name:              scanned.Name,
		SortName:          scanned.SortName,
		ProductionYear:    scanned.Year,
		IndexNumber:       scanned.Index,
		ParentIndexNumber: scanned.ParentIndex,
		DateModified:      newest(scanned.Files),
	})
	if err != nil {
		return err
	}

	for _, file := range scanned.Files {
		jobs.Heartbeat(ctx, file.Path)

		if _, err := s.SaveSource(ctx, MediaSource{
			SourceID:     sourceID,
			ItemID:       item.ID,
			Path:         file.Path,
			Name:         filepath.Base(file.Path),
			DateModified: file.DateModified,
		}); err != nil {
			return err
		}
	}

	if !scanned.Tagged {
		return nil
	}

	return s.SaveMembership(ctx, libraryID, sourceID, []uuid.UUID{item.ID})
}

func (s *Service) parentOf(ctx context.Context, key string) (*uuid.UUID, error) {
	if key == "" {
		return nil, nil
	}

	parent, err := s.store.Item.Query().Where(itemmodal.Key(key)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find the parent %s: %w", key, err)
	}

	return &parent.ID, nil
}

func newest(files []ScannedFile) time.Time {
	latest := time.Time{}
	for _, file := range files {
		if file.DateModified.After(latest) {
			latest = file.DateModified
		}
	}

	return latest
}
