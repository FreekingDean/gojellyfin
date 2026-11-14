package items

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type LatestOrder int

const (
	LatestAdded LatestOrder = iota
	LatestAired
)

const latestWindow = 5

type LatestQuery struct {
	Viewer    Viewer
	LibraryID *uuid.UUID
	Order     LatestOrder
	Now       time.Time
	Limit     int
	Ungrouped bool
}

type Latest struct {
	Item  *Item
	Count int32
}

func (s *Service) LatestItems(ctx context.Context, query LatestQuery) ([]Latest, error) {
	if query.Limit <= 0 {
		query.Limit = 20
	}

	leaves := s.query(query.Viewer).Where(itemmodel.KindIn(itemmodel.KindMovie, itemmodel.KindEpisode))
	if query.LibraryID != nil {
		leaves = leaves.Where(inLibrary(*query.LibraryID))
	}

	switch query.Order {
	case LatestAired:
		leaves = leaves.
			Where(itemmodel.PremiereDateLTE(query.Now)).
			Order(itemmodel.ByPremiereDate(sql.OrderDesc()))
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

	return s.groupBySeries(ctx, records, query.Limit)
}

func byAdded(selector *sql.Selector) {
	selector.OrderExpr(sql.Expr(fmt.Sprintf(
		"COALESCE(%[1]s, %[2]s) DESC, %[2]s DESC",
		selector.C(itemmodel.FieldDateModified),
		selector.C(itemmodel.FieldCreatedAt),
	)))
}

func (s *Service) groupBySeries(ctx context.Context, records []*Item, limit int) ([]Latest, error) {
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
		if parent, ok := series[found.first.ID]; ok && found.count > 1 {
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
