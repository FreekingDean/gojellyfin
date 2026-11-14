package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const similarQuery = `
	SELECT shared.id
	FROM (
		SELECT theirs.item_id AS id, 10 AS weight
		FROM item_genres AS ours
		JOIN item_genres AS theirs ON theirs.genre_id = ours.genre_id
		WHERE ours.item_id = $1
		UNION ALL
		SELECT theirs.item_id, 3
		FROM item_studios AS ours
		JOIN item_studios AS theirs ON theirs.studio_id = ours.studio_id
		WHERE ours.item_id = $1
		UNION ALL
		SELECT people.item_id, 2
		FROM (
			SELECT DISTINCT theirs.item_credits AS item_id, theirs.person_credits
			FROM credits AS ours
			JOIN credits AS theirs ON theirs.person_credits = ours.person_credits
			WHERE ours.item_credits = $1
		) AS people
	) AS shared
	JOIN items ON items.id = shared.id
	WHERE shared.id <> $1 AND items.kind = $2 AND items.deleted_at IS NULL
	GROUP BY shared.id, items.community_rating, items.sort_name
	ORDER BY sum(shared.weight) DESC, items.community_rating DESC NULLS LAST, items.sort_name
	LIMIT $3`

func (s *Service) Similar(ctx context.Context, viewer Viewer, itemID uuid.UUID, limit int) ([]*Item, error) {
	if limit <= 0 {
		limit = 12
	}

	seed, err := s.titleOf(ctx, viewer, itemID)
	if err != nil {
		return nil, err
	}

	rows, err := s.store.QueryContext(ctx, similarQuery, seed.ID, seed.Kind, limit*5)
	if err != nil {
		return nil, fmt.Errorf("failed to rank similar items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]uuid.UUID, 0, limit*5)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan a similar item: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read similar items: %w", err)
	}

	visible, err := s.ItemsByIDs(ctx, viewer, ids)
	if err != nil {
		return nil, err
	}

	similar := make([]*Item, 0, limit)
	for _, id := range ids {
		if record, ok := visible[id]; ok && len(similar) < limit {
			similar = append(similar, record)
		}
	}

	return similar, nil
}

func (s *Service) titleOf(ctx context.Context, viewer Viewer, itemID uuid.UUID) (*Item, error) {
	record, err := s.ItemByID(ctx, viewer, itemID)
	if store.IsNotFound(err) {
		return nil, fmt.Errorf("%w: no item %s", ErrNotFound, itemID)
	}
	if err != nil {
		return nil, err
	}

	for (record.Kind == itemmodel.KindEpisode || record.Kind == itemmodel.KindSeason) && record.ParentID != nil {
		record, err = s.ItemByID(ctx, Everyone, *record.ParentID)
		if err != nil {
			return nil, err
		}
	}

	return record, nil
}
