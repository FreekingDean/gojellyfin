package items

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

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
	Container         string
	Size              int64
	Bitrate           int32
	ProbedAt          *time.Time
	DateModified      time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (s *Service) UpsertItem(ctx context.Context, item *Item) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "library_id"}, {Name: "path"}},
		// run_time_ticks, container, size, bitrate and probed_at belong to the
		// probe, not the scan, and would be clobbered back to zero here.
		DoUpdates: clause.AssignmentColumns([]string{
			"parent_id", "type", "name", "sort_name", "overview", "production_year",
			"index_number", "parent_index_number", "premiere_date",
			"date_modified", "updated_at",
		}),
	}).Create(item).Error
}

func (s *Service) SaveItemMedia(ctx context.Context, item *Item) error {
	return s.db.WithContext(ctx).Model(&Item{}).Where("id = ?", item.ID).Updates(map[string]any{
		"run_time_ticks": item.RunTimeTicks,
		"container":      item.Container,
		"size":           item.Size,
		"bitrate":        item.Bitrate,
		"probed_at":      time.Now(),
	}).Error
}

func (s *Service) ItemByID(ctx context.Context, id uuid.UUID) (*Item, error) {
	var item Item
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *Service) GetItemByPath(ctx context.Context, libraryID uuid.UUID, path string) (*Item, error) {
	var item Item
	if err := s.db.WithContext(ctx).First(&item, "library_id = ? AND path = ?", libraryID, path).Error; err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *Service) ListItemsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]Item, error) {
	var items []Item
	if err := s.db.WithContext(ctx).Where("library_id = ?", libraryID).Order("sort_name").Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (s *Service) ListItemsByParent(ctx context.Context, parentID uuid.UUID) ([]Item, error) {
	var items []Item
	if err := s.db.WithContext(ctx).Where("parent_id = ?", parentID).Order("index_number, sort_name").Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

type ItemQuery struct {
	LibraryID  *uuid.UUID
	ParentID   *uuid.UUID
	TopLevel   bool
	Types      []string
	IDs        []uuid.UUID
	SearchTerm string
	SortBy     []string
	Descending bool
	StartIndex int
	Limit      int
}

var sortColumns = map[string]string{
	"sortname":       "sort_name",
	"name":           "sort_name",
	"premieredate":   "premiere_date",
	"productionyear": "production_year",
	"datecreated":    "created_at",
	"datemodified":   "date_modified",
	"indexnumber":    "index_number",
	"random":         "random()",
}

func (s *Service) QueryItems(ctx context.Context, query ItemQuery) ([]Item, int64, error) {
	db := s.db.WithContext(ctx).Model(&Item{})

	if query.LibraryID != nil {
		db = db.Where("library_id = ?", *query.LibraryID)
	}
	if query.TopLevel {
		db = db.Where("parent_id IS NULL")
	}
	if query.ParentID != nil {
		db = db.Where("parent_id = ?", *query.ParentID)
	}
	if len(query.Types) > 0 {
		db = db.Where("type IN ?", query.Types)
	}
	if len(query.IDs) > 0 {
		db = db.Where("id IN ?", query.IDs)
	}
	if query.SearchTerm != "" {
		db = db.Where("name ILIKE ?", "%"+query.SearchTerm+"%")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	direction := " asc"
	if query.Descending {
		direction = " desc"
	}
	for _, sort := range query.SortBy {
		column, ok := sortColumns[strings.ToLower(sort)]
		if !ok {
			continue
		}
		if column == "random()" {
			db = db.Order(column)
			continue
		}
		db = db.Order(column + direction)
	}
	db = db.Order("sort_name" + direction)

	if query.StartIndex > 0 {
		db = db.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		db = db.Limit(query.Limit)
	}

	var items []Item
	if err := db.Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *Service) CountChildren(ctx context.Context, parentIDs []uuid.UUID) (map[uuid.UUID]int32, error) {
	counts := make(map[uuid.UUID]int32, len(parentIDs))
	if len(parentIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		ParentID uuid.UUID
		Count    int32
	}
	err := s.db.WithContext(ctx).Model(&Item{}).
		Select("parent_id, count(*) as count").
		Where("parent_id IN ?", parentIDs).
		Group("parent_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		counts[row.ParentID] = row.Count
	}

	return counts, nil
}

func (s *Service) DeleteItemsNotInPaths(ctx context.Context, libraryID uuid.UUID, paths []string) error {
	query := s.db.WithContext(ctx).Where("library_id = ?", libraryID)
	if len(paths) > 0 {
		query = query.Where("path NOT IN ?", paths)
	}

	return query.Delete(&Item{}).Error
}

func (s *Service) DistinctYears(ctx context.Context, libraryID *uuid.UUID, types []string) ([]int32, error) {
	db := s.db.WithContext(ctx).Model(&Item{}).
		Where("production_year IS NOT NULL")
	if libraryID != nil {
		db = db.Where("library_id = ?", *libraryID)
	}
	if len(types) > 0 {
		db = db.Where("type IN ?", types)
	}

	var years []int32
	if err := db.Distinct().Order("production_year").Pluck("production_year", &years).Error; err != nil {
		return nil, err
	}

	return years, nil
}
