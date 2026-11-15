package syncplay

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func playlistOf(t *testing.T, count, playing int) (Queue, []uuid.UUID) {
	t.Helper()

	ids := make([]uuid.UUID, count)
	for i := range ids {
		ids[i] = uuid.New()
	}

	queue := newQueue(time.Now())
	queue.setPlaylist(ids, time.Now())
	queue.setPlayingByIndex(playing, time.Now())

	return queue, ids
}

func itemsOf(items []QueueItem) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ItemID)
	}

	return ids
}

func TestQueue(t *testing.T) {
	t.Run("removing items before the playing one keeps it playing", func(t *testing.T) {
		queue, ids := playlistOf(t, 4, 2)
		removed := []uuid.UUID{queue.Sorted[0].PlaylistItemID, queue.Sorted[1].PlaylistItemID}

		if queue.remove(removed, time.Now()) {
			t.Fatal("remove reported the playing item gone")
		}
		if queue.PlayingItemID() != ids[2] || queue.PlayingIndex != 0 {
			t.Fatalf("playing %v at %d, want %v at 0", queue.PlayingItemID(), queue.PlayingIndex, ids[2])
		}
	})

	t.Run("removing the playing item falls back to the one before it", func(t *testing.T) {
		queue, ids := playlistOf(t, 3, 1)

		if !queue.remove([]uuid.UUID{queue.Sorted[1].PlaylistItemID}, time.Now()) {
			t.Fatal("remove did not report the playing item gone")
		}
		if queue.PlayingItemID() != ids[0] {
			t.Fatalf("playing %v, want %v", queue.PlayingItemID(), ids[0])
		}
	})

	t.Run("removing the only item leaves nothing playing", func(t *testing.T) {
		queue, _ := playlistOf(t, 1, 0)

		queue.remove([]uuid.UUID{queue.Sorted[0].PlaylistItemID}, time.Now())

		if queue.isPlaying() {
			t.Fatalf("playing index = %d, want none", queue.PlayingIndex)
		}
	})

	t.Run("moving an item follows the playing one", func(t *testing.T) {
		queue, ids := playlistOf(t, 3, 0)

		if !queue.move(queue.Sorted[2].PlaylistItemID, 0, time.Now()) {
			t.Fatal("move refused a known item")
		}
		if got := itemsOf(queue.Sorted); !slices.Equal(got, []uuid.UUID{ids[2], ids[0], ids[1]}) {
			t.Fatalf("order = %v", got)
		}
		if queue.PlayingItemID() != ids[0] || queue.PlayingIndex != 1 {
			t.Fatalf("playing %v at %d, want %v at 1", queue.PlayingItemID(), queue.PlayingIndex, ids[0])
		}
	})

	t.Run("shuffling puts the playing item first and sorting puts it back", func(t *testing.T) {
		queue, ids := playlistOf(t, 6, 3)

		queue.setShuffleMode(Shuffle, time.Now())

		if queue.PlayingIndex != 0 || queue.PlayingItemID() != ids[3] {
			t.Fatalf("shuffled playing %v at %d, want %v first", queue.PlayingItemID(), queue.PlayingIndex, ids[3])
		}
		shuffledIDs := itemsOf(queue.Shuffled)
		slices.SortFunc(shuffledIDs, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
		if !slices.Equal(shuffledIDs, unique(ids)) {
			t.Fatal("the shuffled playlist is not a permutation of the sorted one")
		}

		queue.setShuffleMode(Sorted, time.Now())

		if queue.PlayingIndex != 3 || queue.Shuffled != nil {
			t.Fatalf("sorted playing index = %d, shuffled = %v", queue.PlayingIndex, queue.Shuffled)
		}
	})

	t.Run("queue next lands right after the playing item", func(t *testing.T) {
		queue, ids := playlistOf(t, 3, 0)
		next := uuid.New()

		queue.queueNext([]uuid.UUID{next}, time.Now())

		if got := itemsOf(queue.Sorted); !slices.Equal(got, []uuid.UUID{ids[0], next, ids[1], ids[2]}) {
			t.Fatalf("order = %v", got)
		}
	})

	t.Run("next stops at the end unless the queue repeats", func(t *testing.T) {
		queue, _ := playlistOf(t, 2, 1)

		if queue.next(time.Now()) {
			t.Fatal("next moved past the end")
		}

		queue.setRepeatMode(RepeatAll, time.Now())

		if !queue.next(time.Now()) || queue.PlayingIndex != 0 {
			t.Fatalf("next with repeat all = index %d, want 0", queue.PlayingIndex)
		}
	})
}
