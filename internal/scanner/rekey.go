package scanner

import (
	"context"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

// Items keyed before the key existed carry the path the migration stood in for
// them. Deriving their real key here, rather than letting the walk create fresh
// rows beside them, is what keeps a resume position and a playlist entry
// attached to the title they were made against: the walk then upserts onto the
// row that is already there.
//
// It runs before the walk because the sweep at the end deletes by key, and it
// costs one indexed lookup once a library has been rescanned.
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

	for _, item := range legacy {
		key, ok := derivedKey(item, byID)
		if !ok {
			continue
		}
		if err := s.items.Rekey(ctx, item.ID, key); err != nil {
			return err
		}
	}

	log.Printf("rekeyed %s: %d items carried forward", library.Name, len(legacy))

	return nil
}

// The name and year the old scan parsed are still on the row, so the key comes
// off the item's own columns rather than off the path it used to be keyed on.
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
