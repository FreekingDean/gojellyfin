package items

import (
	"context"
	"fmt"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
)

func withFile() predicate.Item {
	living := func(predicates ...predicate.Item) predicate.Item {
		return itemmodel.HasChildrenWith(append([]predicate.Item{itemmodel.DeletedAtIsNil()}, predicates...)...)
	}

	return itemmodel.Or(
		itemmodel.HasItemSources(),
		living(itemmodel.HasItemSources()),
		living(living(itemmodel.HasItemSources())),
	)
}

func (s *Service) PlayableKeyed(ctx context.Context, viewer Viewer, keys []string, limit int) ([]*Item, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	records, err := s.query(viewer).
		Where(itemmodel.KeyIn(keys...), withFile()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the playable items by key: %w", err)
	}

	byKey := make(map[string]*Item, len(records))
	for _, record := range records {
		byKey[record.Key] = record
	}

	ordered := make([]*Item, 0, len(records))
	for _, key := range keys {
		record, found := byKey[key]
		if !found {
			continue
		}
		if limit > 0 && len(ordered) == limit {
			break
		}
		ordered = append(ordered, record)
		delete(byKey, key)
	}

	return ordered, nil
}
