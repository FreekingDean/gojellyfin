package items

import (
	"context"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	datamodal "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
)

type (
	Item      = store.Item
	Kind      = itemmodal.Kind
	MediaType = itemmodal.MediaType
)

var (
	ValidKind      = itemmodal.KindValidator
	ValidMediaType = itemmodal.MediaTypeValidator
)

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

// What one pass of the scanner knows about a file. The probe owns the
// remaining columns and must not be clobbered from here.
type Scanned struct {
	LibraryID         uuid.UUID
	ParentID          *uuid.UUID
	Kind              Kind
	Name              string
	SortName          string
	Path              string
	ProductionYear    *int32
	IndexNumber       *int32
	ParentIndexNumber *int32
	DateModified      time.Time
}

var folderKinds = map[Kind]bool{
	itemmodal.KindSeries:           true,
	itemmodal.KindSeason:           true,
	itemmodal.KindFolder:           true,
	itemmodal.KindCollectionFolder: true,
	itemmodal.KindBoxSet:           true,
	itemmodal.KindPlaylistsFolder:  true,
	itemmodal.KindUserRootFolder:   true,
}

func (s *Service) SaveScanned(ctx context.Context, scanned Scanned) (*Item, error) {
	isFolder := folderKinds[scanned.Kind]
	mediaType := itemmodal.MediaTypeVideo
	if isFolder {
		mediaType = itemmodal.MediaTypeUnknown
	}

	id, err := s.store.Item.Create().
		SetLibraryID(scanned.LibraryID).
		SetNillableParentID(scanned.ParentID).
		SetKind(scanned.Kind).
		SetMediaType(mediaType).
		SetIsFolder(isFolder).
		SetName(scanned.Name).
		SetSortName(scanned.SortName).
		SetPath(scanned.Path).
		SetNillableProductionYear(scanned.ProductionYear).
		SetNillableIndexNumber(scanned.IndexNumber).
		SetNillableParentIndexNumber(scanned.ParentIndexNumber).
		SetDateModified(scanned.DateModified).
		OnConflictColumns(itemmodal.FieldLibraryID, itemmodal.FieldPath).
		UpdateParentID().
		UpdateKind().
		UpdateMediaType().
		UpdateIsFolder().
		UpdateName().
		UpdateSortName().
		UpdateProductionYear().
		UpdateIndexNumber().
		UpdateParentIndexNumber().
		UpdateDateModified().
		UpdateUpdatedAt().
		ID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to save scanned item: %w", err)
	}

	return s.ItemByID(ctx, id)
}

func (s *Service) ItemByID(ctx context.Context, id uuid.UUID) (*Item, error) {
	item, err := s.store.Item.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query item: %w", err)
	}

	return item, nil
}

type Ancestry struct {
	Parents   []*Item
	LibraryID uuid.UUID
}

func (s *Service) Ancestors(ctx context.Context, id uuid.UUID) (*Ancestry, error) {
	item, err := s.store.Item.Get(ctx, id)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query item: %w", err)
	}

	ancestry := &Ancestry{Parents: []*Item{}, LibraryID: item.LibraryID}
	seen := map[uuid.UUID]bool{item.ID: true}
	for item.ParentID != nil && !seen[*item.ParentID] {
		parent, err := s.ItemByID(ctx, *item.ParentID)
		if err != nil {
			return nil, err
		}
		seen[parent.ID] = true
		ancestry.Parents = append(ancestry.Parents, parent)
		item = parent
	}

	return ancestry, nil
}

type ItemQuery struct {
	LibraryID  *uuid.UUID
	ParentID   *uuid.UUID
	TopLevel   bool
	Kinds      []Kind
	MediaTypes []MediaType
	IDs        []uuid.UUID
	SearchTerm string
	SortBy     []string
	Descending bool
	StartIndex int
	Limit      int
}

var sortFields = map[string]string{
	"sortname":       itemmodal.FieldSortName,
	"name":           itemmodal.FieldSortName,
	"premieredate":   itemmodal.FieldPremiereDate,
	"productionyear": itemmodal.FieldProductionYear,
	"datecreated":    itemmodal.FieldCreatedAt,
	"datemodified":   itemmodal.FieldDateModified,
	"indexnumber":    itemmodal.FieldIndexNumber,
}

func (s *Service) QueryItems(ctx context.Context, query ItemQuery) ([]*Item, int, error) {
	items := s.store.Item.Query()

	if query.LibraryID != nil {
		items = items.Where(itemmodal.LibraryID(*query.LibraryID))
	}
	if query.TopLevel {
		items = items.Where(itemmodal.ParentIDIsNil())
	}
	if query.ParentID != nil {
		items = items.Where(itemmodal.ParentID(*query.ParentID))
	}
	if len(query.Kinds) > 0 {
		items = items.Where(itemmodal.KindIn(query.Kinds...))
	}
	if len(query.MediaTypes) > 0 {
		items = items.Where(itemmodal.MediaTypeIn(query.MediaTypes...))
	}
	if len(query.IDs) > 0 {
		items = items.Where(itemmodal.IDIn(query.IDs...))
	}
	if query.SearchTerm != "" {
		items = items.Where(itemmodal.NameContainsFold(query.SearchTerm))
	}

	total, err := items.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count items: %w", err)
	}

	direction := sql.OrderAsc()
	if query.Descending {
		direction = sql.OrderDesc()
	}
	for _, sortBy := range query.SortBy {
		if strings.EqualFold(sortBy, "random") {
			items = items.Order(orderRandom)
			continue
		}
		if field, ok := sortFields[strings.ToLower(sortBy)]; ok {
			items = items.Order(sql.OrderByField(field, direction).ToFunc())
		}
	}
	items = items.Order(itemmodal.BySortName(direction))

	if query.StartIndex > 0 {
		items = items.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		items = items.Limit(query.Limit)
	}

	records, err := items.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query items: %w", err)
	}

	return records, total, nil
}

func orderRandom(selector *sql.Selector) {
	selector.OrderExpr(sql.Expr("random()"))
}

func (s *Service) CountChildren(ctx context.Context, parentIDs []uuid.UUID) (map[uuid.UUID]int32, error) {
	counts := make(map[uuid.UUID]int32, len(parentIDs))
	if len(parentIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		ParentID uuid.UUID `json:"parent_id"`
		Count    int       `json:"count"`
	}
	err := s.store.Item.Query().
		Where(itemmodal.ParentIDIn(parentIDs...)).
		GroupBy(itemmodal.FieldParentID).
		Aggregate(store.Count()).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to count children: %w", err)
	}

	for _, row := range rows {
		counts[row.ParentID] = int32(row.Count)
	}

	return counts, nil
}

func (s *Service) DeleteItemsNotInPaths(ctx context.Context, libraryID uuid.UUID, paths []string) error {
	items := s.store.Item.Delete().Where(itemmodal.LibraryID(libraryID))
	if len(paths) > 0 {
		items = items.Where(itemmodal.PathNotIn(paths...))
	}

	if _, err := items.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete missing items: %w", err)
	}

	return nil
}

func (s *Service) DistinctYears(ctx context.Context, libraryID *uuid.UUID, kinds []Kind) ([]int32, error) {
	items := s.store.Item.Query().Where(itemmodal.ProductionYearNotNil())
	if libraryID != nil {
		items = items.Where(itemmodal.LibraryID(*libraryID))
	}
	if len(kinds) > 0 {
		items = items.Where(itemmodal.KindIn(kinds...))
	}

	values, err := items.
		Order(itemmodal.ByProductionYear()).
		Unique(true).
		Select(itemmodal.FieldProductionYear).
		Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct years: %w", err)
	}

	years := make([]int32, 0, len(values))
	for _, value := range values {
		years = append(years, int32(value))
	}

	return years, nil
}

// Items the user started and did not finish, most recently played first. The
// query runs from the user data so the sort column is on the primary table;
// ordering items by the edge makes ent aggregate it away.
func (s *Service) ResumeItems(ctx context.Context, userID uuid.UUID, kinds []Kind, libraryID *uuid.UUID, startIndex, limit int) ([]*Item, int, error) {
	playable := []predicate.Item{itemmodal.IsFolder(false)}
	if len(kinds) > 0 {
		playable = append(playable, itemmodal.KindIn(kinds...))
	}
	if libraryID != nil {
		playable = append(playable, itemmodal.LibraryID(*libraryID))
	}

	data := s.store.UserItemData.Query().
		Where(
			datamodal.UserID(userID),
			datamodal.PlaybackPositionTicksGT(0),
			datamodal.Played(false),
			datamodal.HasItemWith(playable...),
		)

	total, err := data.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count resume items: %w", err)
	}

	data = data.Order(datamodal.ByLastPlayedAt(sql.OrderDesc(), sql.OrderNullsLast()))
	if startIndex > 0 {
		data = data.Offset(startIndex)
	}
	if limit > 0 {
		data = data.Limit(limit)
	}

	rows, err := data.WithItem().All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query resume items: %w", err)
	}

	records := make([]*Item, 0, len(rows))
	for _, row := range rows {
		if row.Edges.Item != nil {
			records = append(records, row.Edges.Item)
		}
	}

	return records, total, nil
}

func (s *Service) CountByKind(ctx context.Context) (map[string]int32, error) {
	var rows []struct {
		Kind  string `json:"kind"`
		Count int    `json:"count"`
	}
	err := s.store.Item.Query().
		GroupBy(itemmodal.FieldKind).
		Aggregate(store.Count()).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to count items by kind: %w", err)
	}

	counts := make(map[string]int32, len(rows))
	for _, row := range rows {
		counts[row.Kind] = int32(row.Count)
	}

	return counts, nil
}
