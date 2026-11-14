package items

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	genremodel "github.com/FreekingDean/gojellyfin/internal/store/genre"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	datamodel "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
)

type LatestOrder int

const (
	LatestAdded LatestOrder = iota
	LatestAired
	LatestRated
)

const latestWindow = 5

type LatestQuery struct {
	Viewer     Viewer
	LibraryID  *uuid.UUID
	Kinds      []Kind
	Genres     []string
	MinRating  *float64
	MaxRunTime *int64
	UnplayedBy uuid.UUID
	Order      LatestOrder
	Now        time.Time
	Limit      int
	Ungrouped  bool
	NewestOnly bool
}

func (q LatestQuery) filters() []predicate.Item {
	kinds := q.Kinds
	if kinds == nil {
		kinds = []Kind{itemmodel.KindMovie, itemmodel.KindEpisode}
	}

	filters := []predicate.Item{itemmodel.KindIn(kinds...)}
	if q.LibraryID != nil {
		filters = append(filters, inLibrary(*q.LibraryID))
	}
	if len(q.Genres) > 0 {
		filters = append(filters, itemmodel.HasGenresWith(genremodel.NameIn(q.Genres...)))
	}
	if q.MinRating != nil {
		filters = append(filters, itemmodel.CommunityRatingGTE(*q.MinRating))
	}
	if q.MaxRunTime != nil {
		filters = append(filters, itemmodel.RunTimeTicksLTE(*q.MaxRunTime))
	}
	if q.UnplayedBy != uuid.Nil {
		filters = append(filters, itemmodel.Not(itemmodel.HasUserDataWith(
			datamodel.UserID(q.UnplayedBy),
			datamodel.Played(true),
		)))
	}

	return filters
}

type Latest struct {
	Item  *Item
	Count int32
}

func (s *Service) LatestItems(ctx context.Context, query LatestQuery) ([]Latest, error) {
	if query.Limit <= 0 {
		query.Limit = 20
	}

	leaves := s.query(query.Viewer).Where(query.filters()...)

	switch query.Order {
	case LatestAired:
		leaves = leaves.
			Where(itemmodel.PremiereDateLTE(query.Now)).
			Order(itemmodel.ByPremiereDate(sql.OrderDesc()))
	case LatestRated:
		leaves = leaves.
			Where(itemmodel.CommunityRatingNotNil()).
			Order(itemmodel.ByCommunityRating(sql.OrderDesc()))
	default:
		leaves = leaves.Order(byAdded)
	}

	window := query.Limit
	if !query.Ungrouped {
		window *= latestWindow
	}

	records, err := leaves.Order(itemmodel.ByID()).Limit(window).AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the latest items: %w", err)
	}

	if query.Ungrouped {
		latest := make([]Latest, 0, len(records))
		for _, record := range records {
			latest = append(latest, Latest{Item: record})
		}

		return latest, nil
	}

	return s.groupBySeries(ctx, records, query.Limit, query.NewestOnly)
}

func byAdded(selector *sql.Selector) {
	selector.OrderExpr(sql.Expr(fmt.Sprintf(
		"COALESCE(%[1]s, %[2]s) DESC, %[2]s DESC",
		selector.C(itemmodel.FieldDateModified),
		selector.C(itemmodel.FieldCreatedAt),
	)))
}

func (s *Service) groupBySeries(ctx context.Context, records []*Item, limit int, newestOnly bool) ([]Latest, error) {
	series, err := s.seriesOf(ctx, records)
	if err != nil {
		return nil, err
	}

	type group struct {
		first *Item
		count int32
	}

	order := make([]uuid.UUID, 0, limit)
	groups := map[uuid.UUID]*group{}
	for _, record := range records {
		key := record.ID
		if parent, ok := series[record.ID]; ok {
			key = parent.ID
		}

		if found, ok := groups[key]; ok {
			found.count++
			continue
		}
		if len(order) == limit {
			continue
		}

		order = append(order, key)
		groups[key] = &group{first: record, count: 1}
	}

	latest := make([]Latest, 0, len(order))
	for _, key := range order {
		found := groups[key]
		if parent, ok := series[found.first.ID]; ok && found.count > 1 && !newestOnly {
			latest = append(latest, Latest{Item: parent, Count: found.count})
			continue
		}
		latest = append(latest, Latest{Item: found.first})
	}

	return latest, nil
}

func (s *Service) seriesOf(ctx context.Context, records []*Item) (map[uuid.UUID]*Item, error) {
	seasonIDs := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		if record.Kind == itemmodel.KindEpisode && record.ParentID != nil {
			seasonIDs = append(seasonIDs, *record.ParentID)
		}
	}

	seasons, err := s.ItemsByIDs(ctx, Everyone, seasonIDs)
	if err != nil {
		return nil, err
	}

	seriesIDs := make([]uuid.UUID, 0, len(seasons))
	for _, season := range seasons {
		if season.ParentID != nil {
			seriesIDs = append(seriesIDs, *season.ParentID)
		}
	}

	parents, err := s.ItemsByIDs(ctx, Everyone, seriesIDs)
	if err != nil {
		return nil, err
	}

	series := make(map[uuid.UUID]*Item, len(records))
	for _, record := range records {
		if record.Kind != itemmodel.KindEpisode || record.ParentID == nil {
			continue
		}
		season, ok := seasons[*record.ParentID]
		if !ok || season.ParentID == nil {
			continue
		}
		if parent, ok := parents[*season.ParentID]; ok && parent.Kind == itemmodel.KindSeries {
			series[record.ID] = parent
		}
	}

	return series, nil
}

func (s *Service) LastPlayed(ctx context.Context, viewer Viewer, userID uuid.UUID, kind Kind) (*Item, []string, error) {
	visible := append([]predicate.Item{itemmodel.DeletedAtIsNil(), itemmodel.KindEQ(kind)}, viewer.visible()...)

	datum, err := s.store.UserItemData.Query().
		Where(
			datamodel.UserID(userID),
			datamodel.Played(true),
			datamodel.LastPlayedAtNotNil(),
			datamodel.HasItemWith(visible...),
		).
		WithItem(func(items *store.ItemQuery) { items.WithGenres() }).
		Order(datamodel.ByLastPlayedAt(sql.OrderDesc())).
		FirstModel(ctx)
	if store.IsNotFound(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find the last played item: %w", err)
	}

	genres := make([]string, 0, len(datum.Item.Genres))
	for _, genre := range datum.Item.Genres {
		genres = append(genres, genre.Name)
	}

	return datum.Item, genres, nil
}
