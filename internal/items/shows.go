package items

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (s *Service) SeriesSeasons(ctx context.Context, seriesID uuid.UUID) ([]*Item, error) {
	records, err := s.query().
		Where(
			itemmodal.KindEQ(itemmodal.KindSeason),
			itemmodal.ParentID(seriesID),
		).
		Order(itemmodal.ByIndexNumber(sql.OrderNullsLast()), itemmodal.BySortName()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query seasons: %w", err)
	}

	return records, nil
}

type EpisodeQuery struct {
	SeriesID   uuid.UUID
	SeasonID   *uuid.UUID
	Season     *int32
	StartIndex int
	Limit      int
}

func (s *Service) SeriesEpisodes(ctx context.Context, query EpisodeQuery) ([]*Item, int, error) {
	episodes := s.query().Where(
		itemmodal.KindEQ(itemmodal.KindEpisode),
		itemmodal.Or(
			itemmodal.ParentID(query.SeriesID),
			itemmodal.HasParentWith(itemmodal.ParentID(query.SeriesID)),
		),
	)
	if query.SeasonID != nil {
		episodes = episodes.Where(itemmodal.ParentID(*query.SeasonID))
	}
	if query.Season != nil {
		episodes = episodes.Where(itemmodal.ParentIndexNumber(*query.Season))
	}

	total, err := episodes.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count episodes: %w", err)
	}

	episodes = episodes.Order(
		itemmodal.ByParentIndexNumber(sql.OrderNullsLast()),
		itemmodal.ByIndexNumber(sql.OrderNullsLast()),
		itemmodal.BySortName(),
	)
	if query.StartIndex > 0 {
		episodes = episodes.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		episodes = episodes.Limit(query.Limit)
	}

	records, err := episodes.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query episodes: %w", err)
	}

	return records, total, nil
}

func (s *Service) UpcomingEpisodes(ctx context.Context, libraryID *uuid.UUID, startIndex, limit int) ([]*Item, int, error) {
	episodes := s.query().Where(
		itemmodal.KindEQ(itemmodal.KindEpisode),
		itemmodal.PremiereDateGT(time.Now()),
	)
	if libraryID != nil {
		episodes = episodes.Where(itemmodal.LibraryID(*libraryID))
	}

	total, err := episodes.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count upcoming episodes: %w", err)
	}

	episodes = episodes.Order(itemmodal.ByPremiereDate(), itemmodal.BySortName())
	if startIndex > 0 {
		episodes = episodes.Offset(startIndex)
	}
	if limit > 0 {
		episodes = episodes.Limit(limit)
	}

	records, err := episodes.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query upcoming episodes: %w", err)
	}

	return records, total, nil
}
