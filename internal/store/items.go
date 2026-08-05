package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

type Item struct {
	ID                uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid()"`
	LibraryID         uuid.UUID  `gorm:"type:uuid;index;uniqueIndex:idx_items_library_path"`
	ParentID          *uuid.UUID `gorm:"type:uuid;index"`
	Type              string     `gorm:"index"`
	Name              string
	SortName          string `gorm:"index"`
	Path              string `gorm:"uniqueIndex:idx_items_library_path"`
	Overview          string
	ProductionYear    *int32
	IndexNumber       *int32
	ParentIndexNumber *int32
	PremiereDate      *time.Time
	RunTimeTicks      *int64
	DateModified      time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (s *storeImpl) UpsertItem(ctx context.Context, item *Item) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "library_id"}, {Name: "path"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"parent_id", "type", "name", "sort_name", "overview", "production_year",
			"index_number", "parent_index_number", "premiere_date", "run_time_ticks",
			"date_modified", "updated_at",
		}),
	}).Create(item).Error
}

func (s *storeImpl) GetItem(ctx context.Context, id uuid.UUID) (*Item, error) {
	var item Item
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *storeImpl) GetItemByPath(ctx context.Context, libraryID uuid.UUID, path string) (*Item, error) {
	var item Item
	if err := s.db.WithContext(ctx).First(&item, "library_id = ? AND path = ?", libraryID, path).Error; err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *storeImpl) ListItemsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]Item, error) {
	var items []Item
	if err := s.db.WithContext(ctx).Where("library_id = ?", libraryID).Order("sort_name").Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (s *storeImpl) ListItemsByParent(ctx context.Context, parentID uuid.UUID) ([]Item, error) {
	var items []Item
	if err := s.db.WithContext(ctx).Where("parent_id = ?", parentID).Order("index_number, sort_name").Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (s *storeImpl) DeleteItemsNotInPaths(ctx context.Context, libraryID uuid.UUID, paths []string) error {
	query := s.db.WithContext(ctx).Where("library_id = ?", libraryID)
	if len(paths) > 0 {
		query = query.Where("path NOT IN ?", paths)
	}

	return query.Delete(&Item{}).Error
}
