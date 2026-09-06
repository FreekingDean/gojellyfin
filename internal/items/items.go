package items

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
	playlistmodal "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	datamodal "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
)

type (
	Item      = store.Item
	Kind      = itemmodal.Kind
	MediaType = playlistmodal.MediaType
)

var (
	ValidKind      = itemmodal.KindValidator
	ValidMediaType = playlistmodal.MediaTypeValidator
)

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

type Scanned struct {
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

var isFolderKind = map[Kind]bool{
	itemmodal.KindSeries:           true,
	itemmodal.KindSeason:           true,
	itemmodal.KindFolder:           true,
	itemmodal.KindCollectionFolder: true,
	itemmodal.KindBoxSet:           true,
	itemmodal.KindPlaylistsFolder:  true,
	itemmodal.KindUserRootFolder:   true,
}

var (
	folderKinds   = []Kind{itemmodal.KindSeries, itemmodal.KindSeason}
	playableKinds = []Kind{itemmodal.KindMovie, itemmodal.KindEpisode}

	audioKinds = []Kind{itemmodal.KindAudio, itemmodal.KindAudioBook}

	allKinds = []Kind{
		itemmodal.KindMovie, itemmodal.KindSeries, itemmodal.KindSeason,
		itemmodal.KindEpisode, itemmodal.KindPlaylist, itemmodal.KindAudio,
		itemmodal.KindAudioBook, itemmodal.KindTrailer, itemmodal.KindVideo,
		itemmodal.KindFolder, itemmodal.KindCollectionFolder, itemmodal.KindBoxSet,
		itemmodal.KindPlaylistsFolder, itemmodal.KindUserRootFolder,
	}
)

func kindsOf(types []MediaType) []Kind {
	wanted := make([]Kind, 0)
	for _, kind := range allKinds {
		if slices.Contains(types, MediaTypeOf(kind)) {
			wanted = append(wanted, kind)
		}
	}

	return wanted
}

func IsFolder(kind Kind) bool {
	return isFolderKind[kind]
}

func MediaTypeOf(kind Kind) MediaType {
	switch {
	case isFolderKind[kind]:
		return playlistmodal.MediaTypeUnknown
	case slices.Contains(audioKinds, kind):
		return playlistmodal.MediaTypeAudio
	default:
		return playlistmodal.MediaTypeVideo
	}
}

func (s *Service) SaveScanned(ctx context.Context, scanned Scanned) (*Item, error) {
	id, err := s.store.Item.Create().
		SetNillableParentID(scanned.ParentID).
		SetKind(scanned.Kind).
		SetKey(scanned.Key).
		SetName(scanned.Name).
		SetSortName(scanned.SortName).
		SetNillableProductionYear(scanned.ProductionYear).
		SetNillableIndexNumber(scanned.IndexNumber).
		SetNillableParentIndexNumber(scanned.ParentIndexNumber).
		SetDateModified(scanned.DateModified).
		OnConflictColumns(itemmodal.FieldKey).
		UpdateParentID().
		UpdateKind().
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

	return s.ItemByID(ctx, Everyone, id)
}

func (s *Service) ItemByID(ctx context.Context, viewer Viewer, id uuid.UUID) (*Item, error) {
	item, err := s.query(viewer).Where(itemmodal.ID(id)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query item: %w", err)
	}

	return item, nil
}

func (s *Service) ItemsByIDs(ctx context.Context, viewer Viewer, ids []uuid.UUID) (map[uuid.UUID]*Item, error) {
	found := make(map[uuid.UUID]*Item, len(ids))
	if len(ids) == 0 {
		return found, nil
	}

	records, err := s.query(viewer).Where(itemmodal.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query items by id: %w", err)
	}

	for _, record := range records {
		found[record.ID] = record
	}

	return found, nil
}

type Ancestry struct {
	Parents    []*Item
	LibraryIDs []uuid.UUID
}

func parsed(values []string) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if id, err := uuid.Parse(value); err == nil {
			ids = append(ids, id)
		}
	}

	return ids
}

func (s *Service) Ancestors(ctx context.Context, id uuid.UUID) (*Ancestry, error) {
	item, err := s.store.Item.Get(ctx, id)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query item: %w", err)
	}

	libraries, err := s.store.LibraryItem.Query().
		Where(librarymembership.ItemID(id)).
		Order(librarymembership.ByLibraryID()).
		Unique(true).
		Select(librarymembership.FieldLibraryID).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the item's libraries: %w", err)
	}

	ancestry := &Ancestry{Parents: []*Item{}, LibraryIDs: parsed(libraries)}
	seen := map[uuid.UUID]bool{item.ID: true}
	for item.ParentID != nil && !seen[*item.ParentID] {
		parent, err := s.ItemByID(ctx, Everyone, *item.ParentID)
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
	query := s.query(Everyone).Where(itemmodal.KindIn(kinds...), itemmodal.LockData(false))
	if !force {
		query = query.Where(itemmodal.ProviderIdsIsNil())
	}
	if scope != uuid.Nil {
		query = query.Where(itemmodal.Or(
			inLibrary(scope),
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
	Viewer Viewer

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
	items := s.query(query.Viewer)

	if query.LibraryID != nil {
		items = items.Where(inLibrary(*query.LibraryID))
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
		items = items.Where(itemmodal.KindIn(kindsOf(query.MediaTypes)...))
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
	err := s.query(Everyone).
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

func (s *Service) query(viewer Viewer) *store.ItemQuery {
	return s.store.Item.Query().
		Where(itemmodal.DeletedAtIsNil()).
		Where(viewer.visible()...)
}

func inLibrary(id uuid.UUID) predicate.Item {
	return itemmodal.HasLibrariesWith(librarymembership.LibraryID(id))
}

type Viewer struct {
	All       bool
	Libraries []uuid.UUID
}

var Everyone = Viewer{All: true}

func (v Viewer) visible() []predicate.Item {
	if v.All {
		return nil
	}

	return []predicate.Item{
		itemmodal.Or(
			itemmodal.KindEQ(itemmodal.KindPlaylist),
			itemmodal.HasLibrariesWith(librarymembership.LibraryIDIn(v.Libraries...)),
		),
	}
}

func (s *Service) SaveMembership(ctx context.Context, libraryID, sourceID uuid.UUID, itemIDs []uuid.UUID) error {
	if len(itemIDs) == 0 {
		return nil
	}

	builders := make([]*store.LibraryItemCreate, 0, len(itemIDs))
	for _, id := range itemIDs {
		builders = append(builders, s.store.LibraryItem.Create().
			SetLibraryID(libraryID).
			SetSourceID(sourceID).
			SetItemID(id))
	}

	err := s.store.LibraryItem.CreateBulk(builders...).
		OnConflictColumns(
			librarymembership.FieldLibraryID,
			librarymembership.FieldItemID,
			librarymembership.FieldSourceID,
		).
		DoNothing().
		Exec(ctx)
	if err != nil && !errors.Is(err, stdsql.ErrNoRows) {
		return fmt.Errorf("failed to save library membership: %w", err)
	}

	return nil
}

func (s *Service) DeleteMembershipNotIn(
	ctx context.Context,
	libraryID, sourceID uuid.UUID,
	itemIDs []uuid.UUID,
) ([]uuid.UUID, error) {
	where := []predicate.LibraryItem{
		librarymembership.LibraryID(libraryID),
		librarymembership.SourceID(sourceID),
	}
	if len(itemIDs) > 0 {
		where = append(where, librarymembership.ItemIDNotIn(itemIDs...))
	}

	dropped, err := s.store.LibraryItem.Query().
		Where(where...).
		Select(librarymembership.FieldItemID).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select stale library membership: %w", err)
	}

	if _, err := s.store.LibraryItem.Delete().Where(where...).Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to drop library membership: %w", err)
	}

	return parsed(dropped), nil
}

func (s *Service) UnreachableItems(ctx context.Context) ([]uuid.UUID, error) {
	orphans, err := s.store.Item.Query().
		Where(
			itemmodal.DeletedAtIsNil(),
			itemmodal.Not(itemmodal.HasLibraries()),
			itemmodal.Not(itemmodal.HasPlaylist()),
		).
		IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query items in no library: %w", err)
	}

	return orphans, nil
}

func (s *Service) SweepUnreachable(ctx context.Context, disturbed []uuid.UUID) error {
	if len(disturbed) == 0 {
		return nil
	}

	if err := s.store.Item.Update().
		Where(
			itemmodal.IDIn(disturbed...),
			itemmodal.DeletedAtIsNil(),
			itemmodal.KindIn(playableKinds...),
			itemmodal.Not(itemmodal.HasItemSources()),
		).
		SetDeletedAt(time.Now()).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to mark fileless items deleted: %w", err)
	}

	for {
		affected, err := s.store.Item.Update().
			Where(
				itemmodal.DeletedAtIsNil(),
				itemmodal.KindIn(folderKinds...),
				itemmodal.HasChildren(),
				itemmodal.Not(itemmodal.HasChildrenWith(itemmodal.DeletedAtIsNil())),
			).
			SetDeletedAt(time.Now()).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark childless folders deleted: %w", err)
		}
		if affected == 0 {
			return nil
		}
	}
}

func (s *Service) DistinctYears(ctx context.Context, viewer Viewer, libraryID *uuid.UUID, kinds []Kind) ([]int32, error) {
	items := s.query(viewer).Where(itemmodal.ProductionYearNotNil())
	if libraryID != nil {
		items = items.Where(inLibrary(*libraryID))
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
	playable := []predicate.Item{itemmodal.KindNotIn(folderKinds...)}
	if len(kinds) > 0 {
		playable = append(playable, itemmodal.KindIn(kinds...))
	}
	if libraryID != nil {
		playable = append(playable, inLibrary(*libraryID))
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
	err := s.query(Everyone).
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
