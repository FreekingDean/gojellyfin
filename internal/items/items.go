package items

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/playlistentry"
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

type Scanned struct {
	LibraryID         uuid.UUID
	ParentID          *uuid.UUID
	Kind              Kind
	Key               string
	Name              string
	SortName          string
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
		SetKey(scanned.Key).
		SetName(scanned.Name).
		SetSortName(scanned.SortName).
		SetNillableProductionYear(scanned.ProductionYear).
		SetNillableIndexNumber(scanned.IndexNumber).
		SetNillableParentIndexNumber(scanned.ParentIndexNumber).
		SetDateModified(scanned.DateModified).
		OnConflictColumns(itemmodal.FieldLibraryID, itemmodal.FieldKey).
		UpdateParentID().
		UpdateKind().
		UpdateMediaType().
		UpdateIsFolder().
		UpdateIndexNumber().
		UpdateParentIndexNumber().
		UpdateDateModified().
		UpdateUpdatedAt().
		ClearDeletedAt().
		Update(unclaimedTitle).
		ID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to save scanned item: %w", err)
	}

	return s.ItemByID(ctx, id)
}

func (s *Service) ItemByID(ctx context.Context, id uuid.UUID) (*Item, error) {
	item, err := s.query().Where(itemmodal.ID(id)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query item: %w", err)
	}

	return item, nil
}

func (s *Service) ItemsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Item, error) {
	found := make(map[uuid.UUID]*Item, len(ids))
	if len(ids) == 0 {
		return found, nil
	}

	records, err := s.query().Where(itemmodal.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query items by id: %w", err)
	}

	for _, record := range records {
		found[record.ID] = record
	}

	return found, nil
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

func (s *Service) ItemsNeedingMetadata(ctx context.Context, kinds []Kind, force bool, scope uuid.UUID) ([]uuid.UUID, error) {
	query := s.query().Where(itemmodal.KindIn(kinds...), itemmodal.LockData(false))
	if !force {
		query = query.Where(itemmodal.ProviderIdsIsNil())
	}
	if scope != uuid.Nil {
		query = query.Where(itemmodal.Or(
			itemmodal.LibraryID(scope),
			itemmodal.ID(scope),
			itemmodal.HasParentWith(itemmodal.ID(scope)),
			itemmodal.HasParentWith(itemmodal.HasParentWith(itemmodal.ID(scope))),
		))
	}

	ranks := make([]string, 0, len(kinds))
	for rank, kind := range kinds {
		ranks = append(ranks, fmt.Sprintf("WHEN '%s' THEN %d", kind, rank))
	}

	ids, err := query.
		Order(func(selector *sql.Selector) {
			selector.OrderExpr(sql.Expr(fmt.Sprintf(
				"CASE %s %s END", selector.C(itemmodal.FieldKind), strings.Join(ranks, " "),
			)))
		}, itemmodal.ByID()).
		IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the items needing metadata: %w", err)
	}

	return ids, nil
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
	items := s.query()

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
	err := s.query().
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

func (s *Service) query() *store.ItemQuery {
	return s.store.Item.Query().Where(itemmodal.DeletedAtIsNil())
}

var ErrNothingScanned = errors.New("items: the scan found no files")

func (s *Service) DeleteItemsNotInKeys(ctx context.Context, libraryID uuid.UUID, keys []string) error {
	if len(keys) == 0 {
		return ErrNothingScanned
	}

	missing := []predicate.Item{
		itemmodal.LibraryID(libraryID),
		itemmodal.DeletedAtIsNil(),
		itemmodal.KeyNotIn(keys...),
	}

	if err := s.store.Item.Update().
		Where(missing...).
		SetDeletedAt(time.Now()).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to mark missing items deleted: %w", err)
	}

	return s.deleteOrphanedDescendants(ctx, libraryID)
}

func (s *Service) deleteOrphanedDescendants(ctx context.Context, libraryID uuid.UUID) error {
	for {
		affected, err := s.store.Item.Update().
			Where(
				itemmodal.LibraryID(libraryID),
				itemmodal.DeletedAtIsNil(),
				itemmodal.HasParentWith(itemmodal.DeletedAtNotNil()),
			).
			SetDeletedAt(time.Now()).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark orphaned items deleted: %w", err)
		}
		if affected == 0 {
			return nil
		}
	}
}

func (s *Service) DistinctYears(ctx context.Context, libraryID *uuid.UUID, kinds []Kind) ([]int32, error) {
	items := s.query().Where(itemmodal.ProductionYearNotNil())
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
	err := s.query().
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

func (s *Service) LegacyKeyedItems(ctx context.Context, libraryID uuid.UUID) ([]*Item, error) {
	records, err := s.query().
		Where(
			itemmodal.LibraryID(libraryID),
			itemmodal.Not(itemmodal.Or(derivedKeys()...)),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query legacy keyed items: %w", err)
	}

	return records, nil
}

func (s *Service) ItemsInLibrary(ctx context.Context, libraryID uuid.UUID) ([]*Item, error) {
	records, err := s.query().Where(itemmodal.LibraryID(libraryID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the items of a library: %w", err)
	}

	return records, nil
}

func (s *Service) Rekey(ctx context.Context, id uuid.UUID, key string) error {
	if err := s.store.Item.UpdateOneID(id).SetKey(key).Exec(ctx); err != nil {
		return fmt.Errorf("failed to rekey %s: %w", id, err)
	}

	return nil
}

func (s *Service) Merge(ctx context.Context, from, into uuid.UUID) error {
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		kept, err := tx.UserItemData.Query().Where(datamodal.ItemID(into)).All(ctx)
		if err != nil {
			return fmt.Errorf("failed to query the surviving item's user data: %w", err)
		}

		survivor := make(map[uuid.UUID]*store.UserItemData, len(kept))
		for _, datum := range kept {
			survivor[datum.UserID] = datum
		}

		folded, err := tx.UserItemData.Query().Where(datamodal.ItemID(from)).All(ctx)
		if err != nil {
			return fmt.Errorf("failed to query the duplicate's user data: %w", err)
		}

		clashed := make([]uuid.UUID, 0, len(folded))
		for _, datum := range folded {
			existing, clash := survivor[datum.UserID]
			if !clash {
				continue
			}
			clashed = append(clashed, datum.UserID)
			if err := union(existing, datum).Exec(ctx); err != nil {
				return fmt.Errorf("failed to fold the duplicate's user data: %w", err)
			}
		}

		if len(clashed) > 0 {
			if _, err := tx.UserItemData.Delete().
				Where(datamodal.ItemID(from), datamodal.UserIDIn(clashed...)).
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to drop the folded user data: %w", err)
			}
		}

		if err := tx.UserItemData.Update().Where(datamodal.ItemID(from)).SetItemID(into).Exec(ctx); err != nil {
			return fmt.Errorf("failed to move the user data: %w", err)
		}
		if err := tx.PlaylistEntry.Update().Where(entrymodal.ItemID(from)).SetItemID(into).Exec(ctx); err != nil {
			return fmt.Errorf("failed to move the playlist entries: %w", err)
		}
		if err := tx.MediaSource.Update().Where(sourcemodal.ItemID(from)).SetItemID(into).Exec(ctx); err != nil {
			return fmt.Errorf("failed to move the media sources: %w", err)
		}
		if err := tx.Item.Update().Where(itemmodal.ParentID(from)).SetParentID(into).Exec(ctx); err != nil {
			return fmt.Errorf("failed to move the children: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return s.DeleteItem(ctx, from)
}

func union(kept, folded *store.UserItemData) *store.UserItemDataUpdateOne {
	update := kept.Update().
		SetPlayed(kept.Played || folded.Played).
		SetIsFavorite(kept.IsFavorite || folded.IsFavorite).
		SetPlayCount(max(kept.PlayCount, folded.PlayCount)).
		SetPlaybackPositionTicks(max(kept.PlaybackPositionTicks, folded.PlaybackPositionTicks))

	if folded.LastPlayedAt != nil && (kept.LastPlayedAt == nil || folded.LastPlayedAt.After(*kept.LastPlayedAt)) {
		update = update.SetLastPlayedAt(*folded.LastPlayedAt)
	}

	return update
}

func derivedKeys() []predicate.Item {
	kinds := []Kind{itemmodal.KindMovie, itemmodal.KindSeries, itemmodal.KindSeason, itemmodal.KindEpisode}
	prefixes := make([]predicate.Item, 0, len(kinds))
	for _, kind := range kinds {
		prefixes = append(prefixes, itemmodal.KeyHasPrefix(strings.ToLower(string(kind))+":"))
	}

	return prefixes
}
