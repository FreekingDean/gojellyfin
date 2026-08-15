package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (s *Service) ItemByName(ctx context.Context, kind Kind, name string) (*Item, error) {
	item, err := s.query().
		Where(itemmodal.KindEQ(kind), itemmodal.NameEqualFold(name)).
		Order(itemmodal.BySortName()).
		First(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query item by name: %w", err)
	}

	return item, nil
}

// Keyed by the id that was asked for, so a page of tracks reaches its albums
// and their artists in one round trip per level.
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
