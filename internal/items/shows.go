package items

import (
	"context"
	"fmt"
	"slices"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (s *Service) SeriesSeasons(ctx context.Context, viewer Viewer, seriesID uuid.UUID) ([]*Item, error) {
	records, err := s.query(viewer).
		Where(
			itemmodel.KindEQ(itemmodel.KindSeason),
			itemmodel.ParentID(seriesID),
		).
		Order(itemmodel.ByIndexNumber(sql.OrderNullsLast()), itemmodel.BySortName()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query seasons: %w", err)
	}

	return records, nil
}

type EpisodeQuery struct {
	Viewer Viewer

	SeriesID    uuid.UUID
	SeasonID    *uuid.UUID
	Season      *int32
	StartItemID *uuid.UUID
	Shuffled    bool
	StartIndex  int
	Limit       int
}

func (s *Service) SeriesEpisodes(ctx context.Context, query EpisodeQuery) ([]*Item, int, error) {
	episodes := s.query(query.Viewer).Where(
		itemmodel.KindEQ(itemmodel.KindEpisode),
		itemmodel.HasParentWith(itemmodel.ParentID(query.SeriesID)),
	)
	if query.SeasonID != nil {
		episodes = episodes.Where(itemmodel.ParentID(*query.SeasonID))
	}
	if query.Season != nil {
		episodes = episodes.Where(itemmodel.ParentIndexNumber(*query.Season))
	}

	if query.Shuffled {
		episodes = episodes.Order(orderRandom)
	} else {
		episodes = episodes.Order(
			itemmodel.ByParentIndexNumber(sql.OrderNullsLast()),
			itemmodel.ByIndexNumber(sql.OrderNullsLast()),
			itemmodel.BySortName(),
		)
	}

	records, err := episodes.AllModels(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query episodes: %w", err)
	}

	if query.StartItemID != nil {
		if start := slices.IndexFunc(records, func(record *Item) bool { return record.ID == *query.StartItemID }); start >= 0 {
			records = records[start:]
		}
	}

	return page(records, query.StartIndex, query.Limit), len(records), nil
}

func page(records []*Item, startIndex, limit int) []*Item {
	records = records[min(startIndex, len(records)):]
	if limit > 0 && limit < len(records) {
		records = records[:limit]
	}

	return records
}

func (s *Service) UpcomingEpisodes(ctx context.Context, viewer Viewer, libraryID *uuid.UUID, startIndex, limit int) ([]*Item, int, error) {
	episodes := s.query(viewer).Where(
		itemmodel.KindEQ(itemmodel.KindEpisode),
		itemmodel.PremiereDateGT(time.Now()),
	)
	if libraryID != nil {
		episodes = episodes.Where(inLibrary(*libraryID))
	}

	total, err := episodes.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count upcoming episodes: %w", err)
	}

	episodes = episodes.Order(itemmodel.ByPremiereDate(), itemmodel.BySortName())
	if startIndex > 0 {
		episodes = episodes.Offset(startIndex)
	}
	if limit > 0 {
		episodes = episodes.Limit(limit)
	}

	records, err := episodes.AllModels(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query upcoming episodes: %w", err)
	}

	return records, total, nil
}
