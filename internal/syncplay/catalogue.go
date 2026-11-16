package syncplay

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Library interface {
	ItemsByIDs(ctx context.Context, viewer items.Viewer, ids []uuid.UUID) (map[uuid.UUID]*items.Item, error)
	FilesByItem(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]items.Held, error)
}

type Policies interface {
	Access(ctx context.Context, id uuid.UUID) (users.Access, error)
}

type known struct {
	runtimes map[uuid.UUID]int64
	hidden   map[uuid.UUID]bool
}

func (k known) runTime(itemID uuid.UUID) int64 {
	return k.runtimes[itemID]
}

func (k known) visible(itemIDs []uuid.UUID) bool {
	for _, id := range itemIDs {
		if _, found := k.runtimes[id]; !found || k.hidden[id] {
			return false
		}
	}

	return true
}

func unique(ids []uuid.UUID) []uuid.UUID {
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })

	return slices.Compact(sorted)
}

func (s *Service) known(ctx context.Context, room *Room, requested []uuid.UUID) (known, error) {
	ids := unique(append(room.Queue.itemIDs(), requested...))
	catalogue := known{runtimes: map[uuid.UUID]int64{}, hidden: map[uuid.UUID]bool{}}
	if len(ids) == 0 {
		return catalogue, nil
	}

	found, err := s.library.ItemsByIDs(ctx, items.Everyone, ids)
	if err != nil {
		return known{}, err
	}

	files, err := s.library.FilesByItem(ctx, ids)
	if err != nil {
		return known{}, err
	}

	for id, item := range found {
		switch {
		case item.RunTimeTicks != nil:
			catalogue.runtimes[id] = *item.RunTimeTicks
		case files[id].RunTimeTicks != nil:
			catalogue.runtimes[id] = *files[id].RunTimeTicks
		default:
			catalogue.runtimes[id] = 0
		}
	}

	checked := map[uuid.UUID]bool{}
	for _, seat := range room.Seats {
		if checked[seat.UserID] {
			continue
		}
		checked[seat.UserID] = true

		seen, err := s.visibleTo(ctx, seat.UserID, ids)
		if err != nil {
			return known{}, err
		}
		for _, id := range ids {
			if _, ok := seen[id]; !ok {
				catalogue.hidden[id] = true
			}
		}
	}

	return catalogue, nil
}

func (s *Service) visibleTo(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]*items.Item, error) {
	access, err := s.policies.Access(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.library.ItemsByIDs(ctx, items.Viewer{All: access.All, Libraries: access.Libraries, MaxRating: access.MaxRating, BlockUnrated: access.BlockUnrated}, ids)
}

func (s *Service) CanSeeQueue(ctx context.Context, userID uuid.UUID, group *Group) (bool, error) {
	ids := make([]uuid.UUID, 0, len(group.Queue))
	for _, entry := range group.Queue {
		ids = append(ids, entry.ItemID)
	}
	ids = unique(ids)
	if len(ids) == 0 {
		return true, nil
	}

	seen, err := s.visibleTo(ctx, userID, ids)
	if err != nil {
		return false, err
	}

	return len(seen) == len(ids), nil
}
