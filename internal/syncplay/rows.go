package syncplay

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	groupmodal "github.com/FreekingDean/gojellyfin/internal/store/syncplaygroup"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/syncplayqueueentry"
)

func withRoom(query *store.SyncPlayGroupQuery) *store.SyncPlayGroupQuery {
	return withParticipants(query).WithQueue(func(entries *store.SyncPlayQueueEntryQuery) {
		entries.Order(entrymodal.BySortedIndex())
	})
}

func lockRoom(ctx context.Context, tx *store.Tx, groupID uuid.UUID) (*Room, error) {
	group, err := withRoom(tx.SyncPlayGroup.Query()).
		Where(groupmodal.ID(groupID)).
		ForUpdate().
		OnlyModel(ctx)
	if store.IsNotFound(err) {
		return nil, ErrNoGroup
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lock syncplay group %s: %w", groupID, err)
	}

	return roomOf(group), nil
}

func roomOf(group *Group) *Room {
	room := &Room{
		ID:              group.ID,
		Name:            group.Name,
		State:           State(group.State),
		PositionTicks:   group.PositionTicks,
		RunTimeTicks:    group.RunTimeTicks,
		LastActivity:    group.LastActivity,
		ResumePlaying:   group.ResumePlaying,
		PositionJumped:  group.PositionJumped,
		IgnoreBuffering: group.IgnoreBuffering,
		WaitDeadline:    group.WaitDeadline,
		Queue: Queue{
			PlayingIndex: group.PlayingIndex,
			ShuffleMode:  ShuffleMode(group.ShuffleMode),
			RepeatMode:   RepeatMode(group.RepeatMode),
			LastChange:   group.QueueChangedAt,
		},
	}
	if group.WaitingFrom != nil {
		from := State(*group.WaitingFrom)
		room.WaitingFrom = &from
	}

	for _, member := range group.Members {
		seat := &Seat{
			MemberID:   member.ID,
			SessionID:  member.SessionID,
			Ping:       member.Ping,
			Buffering:  member.IsBuffering,
			IgnoreWait: member.IgnoreWait,
		}
		if member.Session != nil && member.Session.User != nil {
			seat.UserID = member.Session.User.ID
			seat.UserName = member.Session.User.Name
		}
		room.Seats = append(room.Seats, seat)
	}

	var shuffledEntries []*store.SyncPlayQueueEntryModel
	for _, entry := range group.Queue {
		room.Queue.Sorted = append(room.Queue.Sorted, QueueItem{ItemID: entry.ItemID, PlaylistItemID: entry.ID})
		if entry.ShuffledIndex != nil {
			shuffledEntries = append(shuffledEntries, entry)
		}
	}
	if room.Queue.ShuffleMode == Shuffle {
		slices.SortFunc(shuffledEntries, func(a, b *store.SyncPlayQueueEntryModel) int { return *a.ShuffledIndex - *b.ShuffledIndex })
		for _, entry := range shuffledEntries {
			room.Queue.Shuffled = append(room.Queue.Shuffled, QueueItem{ItemID: entry.ItemID, PlaylistItemID: entry.ID})
		}
	}

	return room
}

func saveRoom(ctx context.Context, tx *store.Tx, room *Room) error {
	update := tx.SyncPlayGroup.UpdateOneID(room.ID).
		SetState(groupmodal.State(room.State)).
		SetPositionTicks(room.PositionTicks).
		SetRunTimeTicks(room.RunTimeTicks).
		SetLastActivity(room.LastActivity).
		SetPlayingIndex(room.Queue.PlayingIndex).
		SetShuffleMode(groupmodal.ShuffleMode(room.Queue.ShuffleMode)).
		SetRepeatMode(groupmodal.RepeatMode(room.Queue.RepeatMode)).
		SetQueueChangedAt(room.Queue.LastChange).
		SetResumePlaying(room.ResumePlaying).
		SetPositionJumped(room.PositionJumped).
		SetIgnoreBuffering(room.IgnoreBuffering).
		SetNillableWaitDeadline(room.WaitDeadline)
	if room.WaitDeadline == nil {
		update.ClearWaitDeadline()
	}
	if room.WaitingFrom == nil {
		update.ClearWaitingFrom()
	} else {
		update.SetWaitingFrom(groupmodal.WaitingFrom(*room.WaitingFrom))
	}
	if err := update.Exec(ctx); err != nil {
		return fmt.Errorf("failed to save syncplay group %s: %w", room.ID, err)
	}

	for _, seat := range room.Seats {
		if !seat.changed || seat.MemberID == uuid.Nil {
			continue
		}
		if err := tx.SyncPlayGroupMember.UpdateOneID(seat.MemberID).
			SetPing(seat.Ping).
			SetIsBuffering(seat.Buffering).
			SetIgnoreWait(seat.IgnoreWait).
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to save syncplay member %s: %w", seat.SessionID, err)
		}
	}

	if !room.Queue.changed {
		return nil
	}

	return saveQueue(ctx, tx, room)
}

func saveQueue(ctx context.Context, tx *store.Tx, room *Room) error {
	if _, err := tx.SyncPlayQueueEntry.Delete().Where(entrymodal.GroupID(room.ID)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear the queue of syncplay group %s: %w", room.ID, err)
	}
	if len(room.Queue.Sorted) == 0 {
		return nil
	}

	shuffledAt := map[uuid.UUID]int{}
	for index, item := range room.Queue.Shuffled {
		shuffledAt[item.PlaylistItemID] = index
	}

	builders := make([]*store.SyncPlayQueueEntryCreate, 0, len(room.Queue.Sorted))
	for index, item := range room.Queue.Sorted {
		builder := tx.SyncPlayQueueEntry.Create().
			SetID(item.PlaylistItemID).
			SetGroupID(room.ID).
			SetItemID(item.ItemID).
			SetSortedIndex(index)
		if at, ok := shuffledAt[item.PlaylistItemID]; ok {
			builder.SetShuffledIndex(at)
		}
		builders = append(builders, builder)
	}

	if err := tx.SyncPlayQueueEntry.CreateBulk(builders...).Exec(ctx); err != nil {
		return fmt.Errorf("failed to save the queue of syncplay group %s: %w", room.ID, err)
	}

	return nil
}
