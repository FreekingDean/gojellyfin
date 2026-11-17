package items

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
	playlistmodel "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	datamodel "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

type (
	Item      = store.ItemModel
	Kind      = itemmodel.Kind
	MediaType = playlistmodel.MediaType
)

var (
	ValidKind      = itemmodel.KindValidator
	ValidMediaType = playlistmodel.MediaTypeValidator
)

type Service struct {
	store        *store.Client
	encodeHeight int32
}

func New(client *store.Client) *Service {
	return &Service{store: client, encodeHeight: transcode.MaxHeight}
}

func (s *Service) EncodeAtMost(height int32) {
	s.encodeHeight = height
}

func (s *Service) encodable(source *MediaSource) bool {
	return Height(source) <= s.encodeHeight
}

var isFolderKind = map[Kind]bool{
	itemmodel.KindSeries:      true,
	itemmodel.KindSeason:      true,
	itemmodel.KindMusicArtist: true,
	itemmodel.KindMusicAlbum:  true,
}

var (
	folderKinds   = []Kind{itemmodel.KindSeries, itemmodel.KindSeason, itemmodel.KindMusicArtist, itemmodel.KindMusicAlbum}
	playableKinds = []Kind{itemmodel.KindMovie, itemmodel.KindEpisode, itemmodel.KindAudio}

	audioKinds = []Kind{itemmodel.KindAudio}

	allKinds = []Kind{
		itemmodel.KindMovie, itemmodel.KindSeries, itemmodel.KindSeason,
		itemmodel.KindEpisode, itemmodel.KindPlaylist, itemmodel.KindMusicArtist,
		itemmodel.KindMusicAlbum, itemmodel.KindAudio,
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
		return playlistmodel.MediaTypeUnknown
	case slices.Contains(audioKinds, kind):
		return playlistmodel.MediaTypeAudio
	default:
		return playlistmodel.MediaTypeVideo
	}
}

func (s *Service) SaveScanned(ctx context.Context, scanned Item) (*Item, error) {
	id, err := s.store.Item.Create().
		SetNillableParentID(scanned.ParentID).
		SetKind(scanned.Kind).
		SetKey(scanned.Key).
		SetName(scanned.Name).
		SetSortName(scanned.SortName).
		SetNillableProductionYear(scanned.ProductionYear).
		SetNillableIndexNumber(scanned.IndexNumber).
		SetNillableParentIndexNumber(scanned.ParentIndexNumber).
		SetNillableDateModified(scanned.DateModified).
		OnConflictColumns(itemmodel.FieldKey).
		UpdateParentID().
		UpdateKind().
		UpdateIndexNumber().
		UpdateParentIndexNumber().
		UpdateDateModified().
		ClearDeletedAt().
		Update(unclaimedTitle).
		ID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to save scanned item: %w", err)
	}

	return s.ItemByID(ctx, Everyone, id)
}

func (s *Service) ItemByID(ctx context.Context, viewer Viewer, id uuid.UUID) (*Item, error) {
	item, err := s.query(viewer).Where(itemmodel.ID(id)).OnlyModel(ctx)
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

	records, err := s.query(viewer).Where(itemmodel.IDIn(ids...)).AllModels(ctx)
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
	item, err := s.store.Item.GetModel(ctx, id)
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

	parents, err := s.parents(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	return &Ancestry{Parents: parents, LibraryIDs: parsed(libraries)}, nil
}

const ancestorsQuery = `
	WITH RECURSIVE chain (id, parent_id, depth) AS (
		SELECT id, parent_id, 0 FROM items WHERE id = $1
		UNION ALL
		SELECT parent.id, parent.parent_id, chain.depth + 1
		FROM items AS parent
		JOIN chain ON parent.id = chain.parent_id
	) CYCLE id SET looped USING path
	SELECT id FROM chain WHERE depth > 0 AND NOT looped ORDER BY depth`

func (s *Service) parents(ctx context.Context, id uuid.UUID) ([]*Item, error) {
	rows, err := s.store.QueryContext(ctx, ancestorsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query the item's parents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []uuid.UUID
	for rows.Next() {
		var parentID uuid.UUID
		if err := rows.Scan(&parentID); err != nil {
			return nil, fmt.Errorf("failed to scan a parent: %w", err)
		}
		if parentID != id {
			ids = append(ids, parentID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the item's parents: %w", err)
	}

	found, err := s.ItemsByIDs(ctx, Everyone, ids)
	if err != nil {
		return nil, err
	}

	parents := make([]*Item, 0, len(ids))
	for _, parentID := range ids {
		parent, ok := found[parentID]
		if !ok {
			return nil, fmt.Errorf("failed to query item: parent %s is gone", parentID)
		}
		parents = append(parents, parent)
	}

	return parents, nil
}

func (s *Service) ItemsNeedingMetadata(ctx context.Context, kinds []Kind, force bool, scope uuid.UUID) ([]uuid.UUID, error) {
	query := s.query(Everyone).Where(itemmodel.KindIn(kinds...), itemmodel.LockData(false))
	if !force {
		query = query.Where(itemmodel.Or(itemmodel.ProviderIdsIsNil(), itemmodel.Not(itemmodel.HasImages())))
	}
	if scope != uuid.Nil {
		query = query.Where(itemmodel.Or(
			inLibrary(scope),
			itemmodel.ID(scope),
			itemmodel.HasParentWith(itemmodel.ID(scope)),
			itemmodel.HasParentWith(itemmodel.HasParentWith(itemmodel.ID(scope))),
		))
	}

	ids, err := query.IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the items needing metadata: %w", err)
	}

	return ids, nil
}

type ItemQuery struct {
	Viewer Viewer

	LibraryID    *uuid.UUID
	ParentID     *uuid.UUID
	TopLevel     bool
	Kinds        []Kind
	ExcludeKinds []Kind
	MediaTypes   []MediaType
	IDs          []uuid.UUID
	ArtistIDs    []uuid.UUID
	AlbumIDs     []uuid.UUID
	SearchTerm   string

	NameStartsWith          string
	NameStartsWithOrGreater string
	NameLessThan            string

	User      uuid.UUID
	Played    *bool
	Favorite  *bool
	Resumable bool
	Folder    *bool

	SortBy     []string
	Descending bool
	StartIndex int
	Limit      *int
}

var sortFields = map[string]string{
	"sortname":       itemmodel.FieldSortName,
	"name":           itemmodel.FieldSortName,
	"premieredate":   itemmodel.FieldPremiereDate,
	"productionyear": itemmodel.FieldProductionYear,
	"datecreated":    itemmodel.FieldCreatedAt,
	"datemodified":   itemmodel.FieldDateModified,
	"indexnumber":    itemmodel.FieldIndexNumber,
}

func (s *Service) QueryItems(ctx context.Context, query ItemQuery) ([]*Item, int, error) {
	items := s.query(query.Viewer).Where(query.structure()...).Where(query.names()...).Where(query.marks()...)

	total, err := items.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count items: %w", err)
	}
	if query.Limit != nil && *query.Limit <= 0 {
		return []*Item{}, total, nil
	}

	items = query.order(items)

	if query.StartIndex > 0 {
		items = items.Offset(query.StartIndex)
	}
	if query.Limit != nil {
		items = items.Limit(*query.Limit)
	}

	records, err := items.AllModels(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query items: %w", err)
	}

	return records, total, nil
}

func (query ItemQuery) structure() []predicate.Item {
	where := []predicate.Item{}
	if query.LibraryID != nil {
		where = append(where, inLibrary(*query.LibraryID))
	}
	if query.TopLevel {
		where = append(where, itemmodel.ParentIDIsNil())
	}
	if query.ParentID != nil {
		where = append(where, itemmodel.ParentID(*query.ParentID))
	}
	if query.Kinds != nil {
		where = append(where, itemmodel.KindIn(query.Kinds...))
	}
	if len(query.ExcludeKinds) > 0 {
		where = append(where, itemmodel.KindNotIn(query.ExcludeKinds...))
	}
	if len(query.MediaTypes) > 0 {
		where = append(where, itemmodel.KindIn(kindsOf(query.MediaTypes)...))
	}
	if len(query.IDs) > 0 {
		where = append(where, itemmodel.IDIn(query.IDs...))
	}
	if len(query.ArtistIDs) > 0 {
		where = append(where, itemmodel.Or(
			itemmodel.And(itemmodel.KindEQ(itemmodel.KindMusicAlbum), itemmodel.ParentIDIn(query.ArtistIDs...)),
			itemmodel.And(itemmodel.KindEQ(itemmodel.KindAudio), itemmodel.HasParentWith(itemmodel.ParentIDIn(query.ArtistIDs...))),
		))
	}
	if len(query.AlbumIDs) > 0 {
		where = append(where, itemmodel.ParentIDIn(query.AlbumIDs...))
	}
	if query.Folder != nil {
		folders := itemmodel.KindIn(folderKinds...)
		if !*query.Folder {
			folders = itemmodel.Not(folders)
		}
		where = append(where, folders)
	}

	return where
}

func (query ItemQuery) names() []predicate.Item {
	where := []predicate.Item{}
	if query.SearchTerm != "" {
		where = append(where, itemmodel.NameContainsFold(query.SearchTerm))
	}
	if query.NameStartsWith != "" {
		where = append(where, itemmodel.SortNameHasPrefix(sorted(query.NameStartsWith)))
	}
	if query.NameStartsWithOrGreater != "" {
		where = append(where, itemmodel.SortNameGTE(sorted(query.NameStartsWithOrGreater)))
	}
	if query.NameLessThan != "" {
		where = append(where, itemmodel.SortNameLT(sorted(query.NameLessThan)))
	}

	return where
}

func (query ItemQuery) marks() []predicate.Item {
	where := []predicate.Item{}
	if query.Played != nil {
		where = append(where, marked(query.User, *query.Played, datamodel.Played(true)))
	}
	if query.Favorite != nil {
		where = append(where, marked(query.User, *query.Favorite, datamodel.IsFavorite(true)))
	}
	if query.Resumable {
		where = append(where, marked(query.User, true, datamodel.PlaybackPositionTicksGT(0)))
	}

	return where
}

func (query ItemQuery) order(items *store.ItemQuery) *store.ItemQuery {
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
	items = items.Order(itemmodel.BySortName(direction))

	return items
}

func (s *Service) NamedItem(ctx context.Context, viewer Viewer, kind Kind, name string) (*Item, error) {
	item, err := s.query(viewer).
		Where(itemmodel.KindEQ(kind), itemmodel.NameEqualFold(name)).
		Order(itemmodel.ByCreatedAt()).
		FirstModel(ctx)
	if store.IsNotFound(err) {
		return nil, fmt.Errorf("%w: no %s is named %q", ErrNotFound, kind, name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find the %s %q: %w", kind, name, err)
	}

	return item, nil
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
		Where(itemmodel.ParentIDIn(parentIDs...)).
		GroupBy(itemmodel.FieldParentID).
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
		Where(itemmodel.DeletedAtIsNil()).
		Where(viewer.visible()...)
}

func inLibrary(id uuid.UUID) predicate.Item {
	return itemmodel.HasLibrariesWith(librarymembership.LibraryID(id))
}

type Viewer struct {
	All          bool
	Libraries    []uuid.UUID
	MaxRating    *int
	BlockUnrated []string
}

var Everyone = Viewer{All: true}

func (v Viewer) Sees(library uuid.UUID) bool {
	return v.All || slices.Contains(v.Libraries, library)
}

func marked(user uuid.UUID, want bool, mark predicate.UserItemData) predicate.Item {
	has := itemmodel.HasUserDataWith(datamodel.UserID(user), mark)
	if want {
		return has
	}

	return itemmodel.Not(has)
}

func (v Viewer) visible() []predicate.Item {
	if v.All {
		return v.rated()
	}

	return append(v.rated(), itemmodel.Or(
		itemmodel.KindEQ(itemmodel.KindPlaylist),
		itemmodel.HasLibrariesWith(librarymembership.LibraryIDIn(v.Libraries...)),
	))
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

func (s *Service) DeleteMembershipNotInKeys(
	ctx context.Context,
	libraryID, sourceID uuid.UUID,
	keys []string,
) ([]uuid.UUID, error) {
	return s.deleteMembershipNotIn(ctx, keys,
		librarymembership.LibraryID(libraryID),
		librarymembership.SourceID(sourceID),
	)
}

func (s *Service) DeleteScopedMembershipNotInKeys(
	ctx context.Context,
	libraryID, sourceID uuid.UUID,
	scope Scope,
	keys []string,
) ([]uuid.UUID, error) {
	return s.deleteMembershipNotIn(ctx, keys,
		librarymembership.LibraryID(libraryID),
		librarymembership.SourceID(sourceID),
		librarymembership.HasItemWith(scope.items()),
	)
}

func (s *Service) deleteMembershipNotIn(ctx context.Context, keys []string, where ...predicate.LibraryItem) ([]uuid.UUID, error) {
	if len(keys) > 0 {
		where = append(where, librarymembership.HasItemWith(itemmodel.KeyNotIn(keys...)))
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
			itemmodel.DeletedAtIsNil(),
			itemmodel.Not(itemmodel.HasLibraries()),
			itemmodel.Not(itemmodel.HasPlaylist()),
		).
		IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query items in no library: %w", err)
	}

	return orphans, nil
}

func (s *Service) SweepUnreported(ctx context.Context, disturbed []uuid.UUID, reported []string) error {
	if len(disturbed) == 0 {
		return nil
	}

	query := s.store.Item.Query().Where(itemmodel.IDIn(disturbed...))
	if len(reported) > 0 {
		query.Where(itemmodel.KeyNotIn(reported...))
	}

	unreported, err := query.IDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to query the titles no longer reported: %w", err)
	}

	return s.SweepUnreachable(ctx, unreported)
}

func (s *Service) SweepUnreachable(ctx context.Context, disturbed []uuid.UUID) error {
	if len(disturbed) == 0 {
		return nil
	}

	if err := s.store.Item.Update().
		Where(
			itemmodel.IDIn(disturbed...),
			itemmodel.DeletedAtIsNil(),
			itemmodel.KindIn(playableKinds...),
			itemmodel.Not(itemmodel.HasItemSources()),
		).
		Modify(store.StampNow(itemmodel.FieldDeletedAt)).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to mark fileless items deleted: %w", err)
	}

	for {
		affected, err := s.store.Item.Update().
			Where(
				itemmodel.DeletedAtIsNil(),
				itemmodel.KindIn(folderKinds...),
				itemmodel.HasChildren(),
				itemmodel.Not(itemmodel.HasChildrenWith(itemmodel.DeletedAtIsNil())),
			).
			Modify(store.StampNow(itemmodel.FieldDeletedAt)).
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
	items := s.query(viewer).Where(itemmodel.ProductionYearNotNil())
	if libraryID != nil {
		items = items.Where(inLibrary(*libraryID))
	}
	if kinds != nil {
		items = items.Where(itemmodel.KindIn(kinds...))
	}

	values, err := items.
		Order(itemmodel.ByProductionYear()).
		Unique(true).
		Select(itemmodel.FieldProductionYear).
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
	playable := []predicate.Item{itemmodel.KindNotIn(folderKinds...)}
	if kinds != nil {
		playable = append(playable, itemmodel.KindIn(kinds...))
	}
	if libraryID != nil {
		playable = append(playable, inLibrary(*libraryID))
	}

	data := s.store.UserItemData.Query().
		Where(
			datamodel.UserID(userID),
			datamodel.PlaybackPositionTicksGT(0),
			datamodel.Played(false),
			datamodel.HasItemWith(playable...),
		)

	total, err := data.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count resume items: %w", err)
	}

	data = data.Order(datamodel.ByLastPlayedAt(sql.OrderDesc(), sql.OrderNullsLast()))
	if startIndex > 0 {
		data = data.Offset(startIndex)
	}
	if limit > 0 {
		data = data.Limit(limit)
	}

	rows, err := data.WithItem().AllModels(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query resume items: %w", err)
	}

	records := make([]*Item, 0, len(rows))
	for _, row := range rows {
		if row.Item != nil {
			records = append(records, row.Item)
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
		GroupBy(itemmodel.FieldKind).
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

func deref[T any](value *T) T {
	if value == nil {
		var zero T

		return zero
	}

	return *value
}
