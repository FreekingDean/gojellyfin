package items

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const (
	ticksPerMinute = 600_000_000
	cutTolerance   = 2 * ticksPerMinute
	cutSeparator   = ":cut"

	palSpeedup       = 25.0 / 24.0
	palSpeedupMargin = 0.005
)

// Two files under one key are two copies of one title, which is how a 4K and a
// 1080p rip become one entry. An extended cut is a different film wearing the
// same name, so it takes a key of its own rather than becoming a version of the
// one already there.
func (s *Service) cutKey(ctx context.Context, scanned Scanned) (string, error) {
	if scanned.RunTimeTicks == 0 {
		return scanned.Key, nil
	}

	cuts, err := s.cutsOf(ctx, scanned.LibraryID, scanned.Key)
	if err != nil {
		return "", err
	}

	claimed := false
	for _, cut := range cuts {
		claimed = claimed || cut.Key == scanned.Key
		if cut.Key == scanned.Key && len(cut.Edges.MediaSources) == 0 {
			return cut.Key, nil
		}
		for _, source := range cut.Edges.MediaSources {
			if sameCut(source.RunTimeTicks, scanned.RunTimeTicks) {
				return cut.Key, nil
			}
		}
	}
	if !claimed {
		return scanned.Key, nil
	}

	return scanned.Key + cutSeparator + strconv.FormatInt(scanned.RunTimeTicks/ticksPerMinute, 10), nil
}

// Soft deleted rows are included, because a returning file has to land back on
// the row that carries its watch state.
func (s *Service) cutsOf(ctx context.Context, libraryID uuid.UUID, key string) ([]*Item, error) {
	cuts, err := s.store.Item.Query().
		Where(
			itemmodal.LibraryID(libraryID),
			itemmodal.Or(
				itemmodal.Key(key),
				itemmodal.KeyHasPrefix(key+cutSeparator),
			),
		).
		WithMediaSources().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the cuts of %s: %w", key, err)
	}

	return cuts, nil
}

// Runtime is the cheap test for whether two files are the same cut. A PAL
// transfer runs 25/24 fast and is still the same cut, which no plain tolerance
// can cover without swallowing the extended ones.
func sameCut(a, b int64) bool {
	if a == 0 || b == 0 {
		return true
	}

	longer, shorter := max(a, b), min(a, b)
	if longer-shorter <= cutTolerance {
		return true
	}

	return math.Abs(float64(longer)/float64(shorter)-palSpeedup) <= palSpeedupMargin
}
