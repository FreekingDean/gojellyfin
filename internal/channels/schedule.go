package channels

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
)

const tick = 100 * time.Nanosecond

type Slot struct {
	Root     uuid.UUID
	ItemID   uuid.UUID
	SourceID uuid.UUID
	Length   time.Duration
}

type Airing struct {
	Slot
	Number int64
	Start  time.Time
	End    time.Time
}

type Schedule struct {
	order Order
	seed  int64
	epoch time.Time
	slots []Slot
	cycle time.Duration
}

func NewSchedule(order Order, seed int64, epoch time.Time, slots []Slot) Schedule {
	schedule := Schedule{order: order, seed: seed, epoch: epoch}
	for _, slot := range slots {
		if slot.Length > 0 {
			schedule.slots = append(schedule.slots, slot)
			schedule.cycle += slot.Length
		}
	}
	if order == channelmodel.OrderInterleave {
		schedule.slots = interleaved(schedule.slots)
	}

	return schedule
}

type turn struct {
	slot  Slot
	group int
	index int
	count int
}

func interleaved(slots []Slot) []Slot {
	groups := map[uuid.UUID]int{}
	counts := []int{}
	turns := make([]turn, len(slots))
	for i, slot := range slots {
		group, seen := groups[slot.Root]
		if !seen {
			group = len(counts)
			groups[slot.Root] = group
			counts = append(counts, 0)
		}
		turns[i] = turn{slot: slot, group: group, index: counts[group]}
		counts[group]++
	}

	for i := range turns {
		turns[i].count = counts[turns[i].group]
	}
	slices.SortStableFunc(turns, func(a, b turn) int {
		if byShare := cmp.Compare(int64(2*a.index+1)*int64(b.count), int64(2*b.index+1)*int64(a.count)); byShare != 0 {
			return byShare
		}

		return cmp.Compare(a.group, b.group)
	})

	ordered := make([]Slot, len(turns))
	for i, each := range turns {
		ordered[i] = each.slot
	}

	return ordered
}

func (s Schedule) Empty() bool {
	return s.cycle == 0
}

func (s Schedule) At(at time.Time) (Airing, bool) {
	airings := s.Between(at, at.Add(time.Nanosecond))
	if len(airings) == 0 {
		return Airing{}, false
	}

	return airings[0], true
}

func (s Schedule) Between(from, to time.Time) []Airing {
	if s.Empty() || !to.After(from) {
		return nil
	}

	cycle := floorDiv(from.Sub(s.epoch), s.cycle)
	start := s.epoch.Add(time.Duration(cycle) * s.cycle)

	airings := []Airing{}
	for start.Before(to) {
		order, offsets := s.ordered(cycle)
		first := 0
		if from.After(start) {
			elapsed := from.Sub(start)
			first = sort.Search(len(offsets), func(i int) bool { return offsets[i]+order[i].Length > elapsed })
		}

		for i := first; i < len(order); i++ {
			began := start.Add(offsets[i])
			if !began.Before(to) {
				break
			}
			airings = append(airings, s.airing(cycle, i, order[i], began))
		}

		cycle++
		start = start.Add(s.cycle)
	}

	return airings
}

func (s Schedule) airing(cycle int64, position int, slot Slot, began time.Time) Airing {
	return Airing{
		Slot:   slot,
		Number: cycle*int64(len(s.slots)) + int64(position),
		Start:  began,
		End:    began.Add(slot.Length),
	}
}

func (s Schedule) ordered(cycle int64) ([]Slot, []time.Duration) {
	order := s.slots
	if s.order == channelmodel.OrderShuffle {
		order = append([]Slot(nil), s.slots...)
		shuffler := rand.New(rand.NewPCG(uint64(s.seed), uint64(cycle)))
		shuffler.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}

	offsets := make([]time.Duration, len(order))
	var elapsed time.Duration
	for i, slot := range order {
		offsets[i] = elapsed
		elapsed += slot.Length
	}

	return order, offsets
}

func floorDiv(elapsed, cycle time.Duration) int64 {
	quotient := int64(elapsed / cycle)
	if elapsed < 0 && elapsed%cycle != 0 {
		quotient--
	}

	return quotient
}
