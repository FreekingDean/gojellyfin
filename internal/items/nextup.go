package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

// DISTINCT ON picks one row per series and the ORDER BY decides which, which
// is why this is not expressed through the query builder.
const nextUpQuery = `
	SELECT DISTINCT ON (series.id) episodes.id
	FROM items AS episodes
	JOIN items AS seasons ON seasons.id = episodes.parent_id
	JOIN items AS series ON series.id = seasons.parent_id
	LEFT JOIN user_item_data AS data
		ON data.item_id = episodes.id AND data.user_id = $1
	WHERE episodes.kind = 'Episode'
		AND COALESCE(data.played, false) = false
		AND COALESCE(episodes.parent_index_number, 0) > 0
		AND ($2::uuid IS NULL OR series.id = $2::uuid)
		AND EXISTS (
			SELECT 1
			FROM items AS watched
			JOIN items AS watched_season ON watched_season.id = watched.parent_id
			JOIN user_item_data AS watched_data
				ON watched_data.item_id = watched.id AND watched_data.user_id = $1
			WHERE watched_season.parent_id = series.id AND watched_data.played
		)
	ORDER BY series.id, episodes.parent_index_number, episodes.index_number
	LIMIT $3`

// The earliest unwatched episode of each series the user has already started.
func (s *Service) NextUpEpisodes(ctx context.Context, userID uuid.UUID, seriesID *uuid.UUID, limit int) ([]*Item, error) {
	if limit <= 0 {
		limit = 24
	}

	rows, err := s.store.QueryContext(ctx, nextUpQuery, userID, seriesID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query next up episodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan next up episode: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read next up episodes: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	records, err := s.query().Where(itemmodal.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load next up episodes: %w", err)
	}

	byID := make(map[uuid.UUID]*Item, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}

	episodes := make([]*Item, 0, len(ids))
	for _, id := range ids {
		if episode, ok := byID[id]; ok {
			episodes = append(episodes, episode)
		}
	}

	return episodes, nil
}
