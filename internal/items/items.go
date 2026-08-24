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
	genremodal "github.com/FreekingDean/gojellyfin/internal/store/genre"
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

// What one pass of the scanner knows about a title. The probe owns the
// remaining columns and must not be clobbered from here.
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
	itemmodal.KindMusicArtist:      true,
	itemmodal.KindMusicAlbum:       true,
}

func scannedMediaType(kind Kind) MediaType {
	switch {
	case folderKinds[kind]:
		return itemmodal.MediaTypeUnknown
	case kind == itemmodal.KindAudio || kind == itemmodal.KindAudioBook:
		return itemmodal.MediaTypeAudio
	default:
		return itemmodal.MediaTypeVideo
	}
}

func (s *Service) SaveScanned(ctx context.Context, scanned Scanned) (*Item, error) {
	isFolder := folderKinds[scanned.Kind]
	mediaType := scannedMediaType(scanned.Kind)

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
		UpdateName().
		UpdateSortName().
		UpdateProductionYear().
		UpdateIndexNumber().
		UpdateParentIndexNumber().
		UpdateDateModified().
		UpdateUpdatedAt().
		// The file is back, so the row it left behind is revived rather than
		// replaced, which is what keeps the watch state attached to it.
		ClearDeletedAt().
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

func (s *Service) UnidentifiedItems(ctx context.Context, kinds []Kind, limit int) ([]*Item, error) {
	records, err := s.query().
		Where(
			itemmodal.KindIn(kinds...),
			itemmodal.LockData(false),
			itemmodal.ProviderIdsIsNil(),
		).
		Order(itemmodal.ByUpdatedAt()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query unidentified items: %w", err)
	}

	return records, nil
}

type ItemQuery struct {
	LibraryID  *uuid.UUID
	ParentID   *uuid.UUID
	TopLevel   bool
	Kinds      []Kind
	ChildKinds []Kind
	MediaTypes []MediaType
	IDs        []uuid.UUID
	ArtistIDs  []uuid.UUID
	AlbumIDs   []uuid.UUID
	GenreIDs   []uuid.UUID
	SearchTerm string
	SortBy     []string
	Descending bool
	StartIndex int
	Limit      int
}

var sortFields = map[string]string{
	"sortname":          itemmodal.FieldSortName,
	"name":              itemmodal.FieldSortName,
	"premieredate":      itemmodal.FieldPremiereDate,
	"productionyear":    itemmodal.FieldProductionYear,
	"datecreated":       itemmodal.FieldCreatedAt,
	"datemodified":      itemmodal.FieldDateModified,
	"indexnumber":       itemmodal.FieldIndexNumber,
	"parentindexnumber": itemmodal.FieldParentIndexNumber,
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
	if len(query.ChildKinds) > 0 {
		items = items.Where(itemmodal.HasChildrenWith(
			itemmodal.KindIn(query.ChildKinds...),
			itemmodal.DeletedAtIsNil(),
		))
	}
	if len(query.MediaTypes) > 0 {
		items = items.Where(itemmodal.MediaTypeIn(query.MediaTypes...))
	}
	if len(query.IDs) > 0 {
		items = items.Where(itemmodal.IDIn(query.IDs...))
	}
	if len(query.AlbumIDs) > 0 {
		items = items.Where(itemmodal.ParentIDIn(query.AlbumIDs...))
	}
	// An artist owns its albums and, through them, its tracks, so one id
	// answers for both without a second query.
	if len(query.ArtistIDs) > 0 {
		items = items.Where(itemmodal.Or(
			itemmodal.ParentIDIn(query.ArtistIDs...),
			itemmodal.HasParentWith(itemmodal.ParentIDIn(query.ArtistIDs...)),
		))
	}
	if len(query.GenreIDs) > 0 {
		items = items.Where(itemmodal.HasGenresWith(genremodal.IDIn(query.GenreIDs...)))
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

// Every read goes through here, so a soft deleted row cannot come back in one
// query because a caller forgot the predicate.
func (s *Service) query() *store.ItemQuery {
	return s.store.Item.Query().Where(itemmodal.DeletedAtIsNil())
}

// A library that scanned to nothing is indistinguishable from one whose storage
// went away, and the second is the common case, so neither sweeps the library.
var ErrNothingScanned = errors.New("items: the scan found no files")

// Soft, so that a volume which comes back brings its watch state with it. The
// row keeps the id the user data hangs off, which is the whole reason a hard
// delete cannot be undone.
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

// A parent going away takes its children, which the foreign key did on a hard
// delete and cannot do on an update. Repeated because a series reaches its
// episodes through its seasons.
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

// Items carrying the identity the key migration stood in for them: the path the
// row used to be keyed on, which no derivation would ever produce. Empty once a
// library has been rescanned, so the scan pays one indexed lookup to find that
// out.
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

// Every item of a library, which the rekey needs so a season can read the name
// and year off the series it hangs under.
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

// Two rows that derive one key are the two copies of one title the key exists
// to collapse, so the second is folded into the first rather than refused: its
// files, its children and its playlist entries move over, and its watch state
// moves wherever the survivor has none of its own. Refusing would leave a
// library that can never scan again, because the collision is on a unique index
// the next run hits just as hard.
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

// Neither row is more true than the other, so the merged datum is whichever
// answer says the title was watched: played and favourite carry, and the counts
// and the position take the further of the two.
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

// A derived key names the kind it belongs to; the migration's stand-in is a
// path and names nothing, which is what tells the two apart.
func derivedKeys() []predicate.Item {
	kinds := []Kind{
		itemmodal.KindMovie, itemmodal.KindSeries, itemmodal.KindSeason, itemmodal.KindEpisode,
		itemmodal.KindMusicArtist, itemmodal.KindMusicAlbum, itemmodal.KindAudio,
	}
	prefixes := make([]predicate.Item, 0, len(kinds))
	for _, kind := range kinds {
		prefixes = append(prefixes, itemmodal.KeyHasPrefix(strings.ToLower(string(kind))+":"))
	}

	return prefixes
}
