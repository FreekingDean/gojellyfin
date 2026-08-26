package scanner

import (
	"context"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (s *Scanner) rekeyLegacy(ctx context.Context, library *libraries.Library) error {
	legacy, err := s.items.LegacyKeyedItems(ctx, library.ID)
	if err != nil || len(legacy) == 0 {
		return err
	}

	all, err := s.items.ItemsInLibrary(ctx, library.ID)
	if err != nil {
		return err
	}

	byID := make(map[uuid.UUID]*items.Item, len(all))
	for _, item := range all {
		byID[item.ID] = item
	}

	stale := make(map[uuid.UUID]bool, len(legacy))
	for _, item := range legacy {
		stale[item.ID] = true
	}

	holder := make(map[string]uuid.UUID, len(all))
	for _, item := range all {
		if !stale[item.ID] {
			holder[item.Key] = item.ID
		}
	}

	carried, merged := 0, 0
	for _, item := range legacy {
		jobs.Heartbeat(ctx, item.Key)

		key, ok := derivedKey(item, byID)
		if !ok {
			continue
		}
		if existing, taken := holder[key]; taken {
			if err := s.items.Merge(ctx, item.ID, existing); err != nil {
				return err
			}
			merged++

			continue
		}
		if err := s.items.Rekey(ctx, item.ID, key); err != nil {
			return err
		}
		holder[key] = item.ID
		carried++
	}

	log.Printf("rekeyed %s: %d items carried forward, %d merged into a copy of the same title", library.Name, carried, merged)

	return nil
}

func derivedKey(item *items.Item, byID map[uuid.UUID]*items.Item) (string, bool) {
	switch item.Kind {
	case itemmodal.KindMovie:
		return movieKey(item.Name, item.ProductionYear), true
	case itemmodal.KindSeries:
		return seriesKey(titleSlug(item.Name, item.ProductionYear)), true
	case itemmodal.KindSeason:
		slug, ok := seriesSlug(item, byID)
		if !ok {
			return "", false
		}

		return seasonKey(slug, item.IndexNumber), true
	case itemmodal.KindEpisode:
		slug, ok := seriesSlug(item, byID)
		if !ok {
			return "", false
		}

		return episodeKey(slug, item.ParentIndexNumber, item.IndexNumber, item.Name), true
	default:
		return "", false
	}
}

func seriesSlug(item *items.Item, byID map[uuid.UUID]*items.Item) (string, bool) {
	for parent := item.ParentID; parent != nil; {
		record, ok := byID[*parent]
		if !ok {
			return "", false
		}
		if record.Kind == itemmodal.KindSeries {
			return titleSlug(record.Name, record.ProductionYear), true
		}
		parent = record.ParentID
	}

	return "", false
}
