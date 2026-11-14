package syncplay

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"
)

const noPlayingItem = -1

type ShuffleMode string

const (
	Sorted  ShuffleMode = "Sorted"
	Shuffle ShuffleMode = "Shuffle"
)

type RepeatMode string

const (
	RepeatOne  RepeatMode = "RepeatOne"
	RepeatAll  RepeatMode = "RepeatAll"
	RepeatNone RepeatMode = "RepeatNone"
)

type QueueMode string

const (
	QueueLast QueueMode = "Queue"
	QueueNext QueueMode = "QueueNext"
)

type QueueItem struct {
	ItemID         uuid.UUID `json:"ItemId"`
	PlaylistItemID uuid.UUID `json:"PlaylistItemId"`
}

type Queue struct {
	Sorted       []QueueItem
	Shuffled     []QueueItem
	PlayingIndex int
	ShuffleMode  ShuffleMode
	RepeatMode   RepeatMode
	LastChange   time.Time

	changed bool
}

func newQueue(now time.Time) Queue {
	return Queue{PlayingIndex: noPlayingItem, ShuffleMode: Sorted, RepeatMode: RepeatNone, LastChange: now}
}

func (q *Queue) touch(now time.Time) {
	q.LastChange = now
	q.changed = true
}

func (q *Queue) current() []QueueItem {
	if q.ShuffleMode == Shuffle {
		return q.Shuffled
	}

	return q.Sorted
}

func (q *Queue) setCurrent(playlist []QueueItem) {
	if q.ShuffleMode == Shuffle {
		q.Shuffled = playlist
		return
	}

	q.Sorted = playlist
}

func (q *Queue) Playlist() []QueueItem {
	return slices.Clone(q.current())
}

func (q *Queue) playing() *QueueItem {
	if q.PlayingIndex == noPlayingItem {
		return nil
	}

	return &q.current()[q.PlayingIndex]
}

func (q *Queue) isPlaying() bool {
	return q.PlayingIndex != noPlayingItem
}

func (q *Queue) PlayingPlaylistID() uuid.UUID {
	if item := q.playing(); item != nil {
		return item.PlaylistItemID
	}

	return uuid.Nil
}

func (q *Queue) PlayingItemID() uuid.UUID {
	if item := q.playing(); item != nil {
		return item.ItemID
	}

	return uuid.Nil
}

func (q *Queue) reset(now time.Time) {
	q.Sorted = nil
	q.Shuffled = nil
	q.PlayingIndex = noPlayingItem
	q.ShuffleMode = Sorted
	q.RepeatMode = RepeatNone
	q.touch(now)
}

func queueItems(ids []uuid.UUID) []QueueItem {
	created := make([]QueueItem, 0, len(ids))
	for _, id := range ids {
		created = append(created, QueueItem{ItemID: id, PlaylistItemID: uuid.New()})
	}

	return created
}

func shuffled(items []QueueItem) []QueueItem {
	mixed := slices.Clone(items)
	rand.Shuffle(len(mixed), func(i, j int) { mixed[i], mixed[j] = mixed[j], mixed[i] })

	return mixed
}

func (q *Queue) setPlaylist(ids []uuid.UUID, now time.Time) {
	q.Sorted = queueItems(ids)
	q.Shuffled = nil
	if q.ShuffleMode == Shuffle {
		q.Shuffled = shuffled(q.Sorted)
	}
	q.PlayingIndex = noPlayingItem
	q.touch(now)
}

func (q *Queue) queue(ids []uuid.UUID, now time.Time) {
	added := queueItems(ids)
	q.Sorted = append(q.Sorted, added...)
	if q.ShuffleMode == Shuffle {
		q.Shuffled = append(q.Shuffled, added...)
	}
	q.touch(now)
}

func (q *Queue) queueNext(ids []uuid.UUID, now time.Time) {
	added := queueItems(ids)
	if q.ShuffleMode == Shuffle {
		sortedIndex := slices.IndexFunc(q.Sorted, func(item QueueItem) bool {
			return item.PlaylistItemID == q.PlayingPlaylistID()
		})
		q.Sorted = slices.Insert(q.Sorted, sortedIndex+1, added...)
		q.Shuffled = slices.Insert(q.Shuffled, q.PlayingIndex+1, added...)
	} else {
		q.Sorted = slices.Insert(q.Sorted, q.PlayingIndex+1, added...)
	}
	q.touch(now)
}

func (q *Queue) shufflePlaylist(now time.Time) {
	switch {
	case q.PlayingIndex == noPlayingItem:
		q.Shuffled = shuffled(q.Sorted)
	case q.ShuffleMode == Sorted:
		playing := q.Sorted[q.PlayingIndex]
		q.Shuffled = append([]QueueItem{playing}, shuffled(slices.Delete(slices.Clone(q.Sorted), q.PlayingIndex, q.PlayingIndex+1))...)
		q.PlayingIndex = 0
	default:
		playing := q.Shuffled[q.PlayingIndex]
		q.Shuffled = append([]QueueItem{playing}, shuffled(slices.Delete(slices.Clone(q.Shuffled), q.PlayingIndex, q.PlayingIndex+1))...)
		q.PlayingIndex = 0
	}
	q.ShuffleMode = Shuffle
	q.touch(now)
}

func (q *Queue) restoreSorted(now time.Time) {
	if q.PlayingIndex != noPlayingItem && len(q.Shuffled) > 0 {
		playing := q.Shuffled[q.PlayingIndex]
		q.PlayingIndex = slices.Index(q.Sorted, playing)
	}
	q.Shuffled = nil
	q.ShuffleMode = Sorted
	q.touch(now)
}

func (q *Queue) setShuffleMode(mode ShuffleMode, now time.Time) {
	if mode == Shuffle {
		q.shufflePlaylist(now)
		return
	}

	q.restoreSorted(now)
}

func (q *Queue) setRepeatMode(mode RepeatMode, now time.Time) {
	q.RepeatMode = mode
	q.touch(now)
}

func (q *Queue) clear(clearPlayingItem bool, now time.Time) {
	playing := q.playing()
	var kept *QueueItem
	if playing != nil {
		copied := *playing
		kept = &copied
	}

	q.Sorted = nil
	q.Shuffled = nil
	q.touch(now)

	if clearPlayingItem || kept == nil {
		q.PlayingIndex = noPlayingItem
		return
	}

	q.Sorted = []QueueItem{*kept}
	if q.ShuffleMode == Shuffle {
		q.Shuffled = []QueueItem{*kept}
	}
	q.PlayingIndex = 0
}

func (q *Queue) setPlayingByIndex(index int, now time.Time) {
	if index < 0 || index >= len(q.current()) {
		q.PlayingIndex = noPlayingItem
	} else {
		q.PlayingIndex = index
	}
	q.touch(now)
}

func (q *Queue) setPlayingByPlaylistID(playlistItemID uuid.UUID, now time.Time) bool {
	q.PlayingIndex = slices.IndexFunc(q.current(), func(item QueueItem) bool { return item.PlaylistItemID == playlistItemID })
	q.touch(now)

	return q.PlayingIndex != noPlayingItem
}

func (q *Queue) remove(playlistItemIDs []uuid.UUID, now time.Time) bool {
	removed := func(item QueueItem) bool { return slices.Contains(playlistItemIDs, item.PlaylistItemID) }

	playing := q.playing()
	var kept QueueItem
	removedBefore := 0
	if playing != nil {
		kept = *playing
		for _, item := range q.current()[:q.PlayingIndex] {
			if removed(item) {
				removedBefore++
			}
		}
	}

	q.Sorted = slices.DeleteFunc(q.Sorted, removed)
	q.Shuffled = slices.DeleteFunc(q.Shuffled, removed)
	q.touch(now)

	if playing == nil {
		return false
	}

	if removed(kept) {
		q.PlayingIndex -= removedBefore + 1
		if q.PlayingIndex < 0 {
			q.PlayingIndex = noPlayingItem
			if len(q.current()) > 0 {
				q.PlayingIndex = 0
			}
		}

		return true
	}

	q.setPlayingByPlaylistID(kept.PlaylistItemID, now)

	return false
}

func (q *Queue) move(playlistItemID uuid.UUID, newIndex int, now time.Time) bool {
	playlist := slices.Clone(q.current())
	playing := q.playing()
	var kept QueueItem
	if playing != nil {
		kept = *playing
	}

	oldIndex := slices.IndexFunc(playlist, func(item QueueItem) bool { return item.PlaylistItemID == playlistItemID })
	if oldIndex < 0 {
		return false
	}

	moved := playlist[oldIndex]
	playlist = slices.Delete(playlist, oldIndex, oldIndex+1)
	newIndex = min(max(newIndex, 0), len(playlist))
	playlist = slices.Insert(playlist, newIndex, moved)
	q.setCurrent(playlist)
	q.touch(now)

	q.PlayingIndex = noPlayingItem
	if playing != nil {
		q.PlayingIndex = slices.Index(playlist, kept)
	}

	return true
}

func (q *Queue) next(now time.Time) bool {
	if len(q.current()) == 0 {
		return false
	}
	if q.RepeatMode == RepeatOne {
		q.touch(now)
		return true
	}

	q.PlayingIndex++
	if q.PlayingIndex >= len(q.Sorted) {
		if q.RepeatMode != RepeatAll {
			q.PlayingIndex = len(q.Sorted) - 1
			return false
		}
		q.PlayingIndex = 0
	}
	q.touch(now)

	return true
}

func (q *Queue) previous(now time.Time) bool {
	if len(q.current()) == 0 {
		return false
	}
	if q.RepeatMode == RepeatOne {
		q.touch(now)
		return true
	}

	q.PlayingIndex--
	if q.PlayingIndex < 0 {
		if q.RepeatMode != RepeatAll {
			q.PlayingIndex = 0
			return false
		}
		q.PlayingIndex = len(q.Sorted) - 1
	}
	q.touch(now)

	return true
}

func (q *Queue) itemIDs() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(q.Sorted))
	for _, item := range q.Sorted {
		ids = append(ids, item.ItemID)
	}

	return ids
}
