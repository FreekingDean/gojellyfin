package channels

import (
	"testing"
	"time"

	"github.com/google/uuid"

	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
)

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func slotsOf(lengths ...time.Duration) []Slot {
	slots := make([]Slot, len(lengths))
	for i, length := range lengths {
		slots[i] = Slot{ItemID: uuid.New(), SourceID: uuid.New(), Length: length}
	}

	return slots
}

func TestSchedule_SequentialLoops(t *testing.T) {
	slots := slotsOf(30*time.Minute, time.Hour, 30*time.Minute)
	schedule := NewSchedule(channelmodel.OrderSequential, 1, epoch, slots)

	cases := []struct {
		at    time.Time
		item  uuid.UUID
		start time.Time
	}{
		{epoch, slots[0].ItemID, epoch},
		{epoch.Add(45 * time.Minute), slots[1].ItemID, epoch.Add(30 * time.Minute)},
		{epoch.Add(2*time.Hour + 10*time.Minute), slots[0].ItemID, epoch.Add(2 * time.Hour)},
		{epoch.Add(-10 * time.Minute), slots[2].ItemID, epoch.Add(-30 * time.Minute)},
	}
	for _, c := range cases {
		airing, found := schedule.At(c.at)
		if !found || airing.ItemID != c.item || !airing.Start.Equal(c.start) {
			t.Errorf("At(%s) = %v %s, want %v %s", c.at, airing.ItemID, airing.Start, c.item, c.start)
		}
	}
}

func TestSchedule_BetweenIsContiguous(t *testing.T) {
	schedule := NewSchedule(channelmodel.OrderShuffle, 7, epoch, slotsOf(22*time.Minute, 41*time.Minute, 95*time.Minute, 23*time.Minute))

	from := epoch.Add(1000 * time.Hour)
	airings := schedule.Between(from, from.Add(24*time.Hour))
	if len(airings) == 0 {
		t.Fatal("no airings")
	}
	if airings[0].Start.After(from) || !airings[0].End.After(from) {
		t.Errorf("first airing %s-%s does not cover %s", airings[0].Start, airings[0].End, from)
	}
	for i := 1; i < len(airings); i++ {
		if !airings[i].Start.Equal(airings[i-1].End) {
			t.Errorf("airing %d starts at %s, the last ended at %s", i, airings[i].Start, airings[i-1].End)
		}
	}
	if last := airings[len(airings)-1]; !last.Start.Before(from.Add(24 * time.Hour)) {
		t.Errorf("last airing starts at %s, after the window", last.Start)
	}
}

func TestSchedule_ShuffleIsStableAndReshufflesEachLoop(t *testing.T) {
	slots := slotsOf(time.Minute, time.Minute, time.Minute, time.Minute, time.Minute, time.Minute, time.Minute, time.Minute)
	schedule := NewSchedule(channelmodel.OrderShuffle, 42, epoch, slots)

	first := schedule.Between(epoch, epoch.Add(8*time.Minute))
	again := NewSchedule(channelmodel.OrderShuffle, 42, epoch, slots).Between(epoch, epoch.Add(8*time.Minute))
	second := schedule.Between(epoch.Add(8*time.Minute), epoch.Add(16*time.Minute))

	seen := map[uuid.UUID]bool{}
	differs := false
	for i := range first {
		if first[i].ItemID != again[i].ItemID {
			t.Errorf("airing %d differs between two schedules with one seed", i)
		}
		if first[i].ItemID != second[i].ItemID {
			differs = true
		}
		seen[first[i].ItemID] = true
	}
	if len(seen) != len(slots) {
		t.Errorf("a loop aired %d distinct titles, want %d", len(seen), len(slots))
	}
	if !differs {
		t.Error("the second loop aired in the same order as the first")
	}
}

func TestSchedule_EmptyAirsNothing(t *testing.T) {
	schedule := NewSchedule(channelmodel.OrderShuffle, 1, epoch, slotsOf(0))
	if !schedule.Empty() {
		t.Error("a schedule of zero-length slots is not empty")
	}
	if _, found := schedule.At(epoch); found {
		t.Error("an empty schedule airs something")
	}
}

func TestProgramID_RoundTrips(t *testing.T) {
	start := epoch.Add(90*time.Minute + 123*time.Millisecond)
	number, parsed, ok := ParseProgram(ProgramID(512, start))
	if !ok || number != 512 || !parsed.Equal(start) {
		t.Errorf("ParseProgram = %d %s %v", number, parsed, ok)
	}
	if _, _, ok := ParseProgram(uuid.New()); ok {
		t.Error("a random id parsed as a program")
	}
}
