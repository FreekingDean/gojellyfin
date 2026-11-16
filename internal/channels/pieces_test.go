package channels

import (
	"testing"
	"time"

	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
)

const step = 6 * time.Second

func TestCount(t *testing.T) {
	cases := []struct {
		length time.Duration
		want   int
	}{
		{time.Second, 1},
		{step, 1},
		{step + time.Millisecond, 2},
		{10 * step, 10},
	}
	for _, c := range cases {
		if got := Count(c.length, step); got != c.want {
			t.Errorf("Count(%s) = %d, want %d", c.length, got, c.want)
		}
	}
}

func TestSchedule_PiecesFollowOneAnother(t *testing.T) {
	schedule := NewSchedule(channelmodel.OrderShuffle, 3, epoch, slotsOf(61*time.Second, 25*time.Second, 6*time.Second, 2*time.Minute+500*time.Millisecond))

	from := epoch.Add(97 * time.Hour)
	pieces := schedule.Pieces(step, from, from.Add(20*time.Minute))
	if len(pieces) == 0 {
		t.Fatal("no pieces")
	}
	if pieces[0].Begins.After(from) || !pieces[0].Ends.After(from) {
		t.Errorf("the first piece %s-%s does not cover %s", pieces[0].Begins, pieces[0].Ends, from)
	}

	for i, piece := range pieces {
		if piece.Length() <= 0 || piece.Length() > step {
			t.Errorf("piece %d is %s long", i, piece.Length())
		}
		if i == 0 {
			continue
		}
		last := pieces[i-1]
		if !piece.Begins.Equal(last.Ends) {
			t.Errorf("piece %d begins at %s, the last ended at %s", i, piece.Begins, last.Ends)
		}
		if piece.Sequence != last.Sequence+1 {
			t.Errorf("piece %d is number %d, the last was %d", i, piece.Sequence, last.Sequence)
		}
		switch {
		case piece.Number == last.Number && piece.Index != last.Index+1:
			t.Errorf("piece %d is index %d of its airing, the last was %d", i, piece.Index, last.Index)
		case piece.Number != last.Number && (piece.Number != last.Number+1 || piece.Index != 0):
			t.Errorf("piece %d opens airing %d at index %d after airing %d", i, piece.Number, piece.Index, last.Number)
		}
	}
}

func TestSchedule_PieceIsFoundByItsSequence(t *testing.T) {
	schedule := NewSchedule(channelmodel.OrderShuffle, 11, epoch, slotsOf(61*time.Second, 25*time.Second, 2*time.Minute))

	for _, from := range []time.Time{epoch, epoch.Add(5 * time.Hour), epoch.Add(-3 * time.Hour)} {
		for _, piece := range schedule.Pieces(step, from, from.Add(10*time.Minute)) {
			found, ok := schedule.Piece(step, piece.Sequence)
			if !ok {
				t.Fatalf("piece %d was not found", piece.Sequence)
			}
			if found.Number != piece.Number || found.Index != piece.Index || !found.Begins.Equal(piece.Begins) || found.ItemID != piece.ItemID {
				t.Errorf("piece %d is %+v, listed as %+v", piece.Sequence, found, piece)
			}
		}
	}
}

func TestSchedule_AiringsAreNumberedInOrder(t *testing.T) {
	schedule := NewSchedule(channelmodel.OrderSequential, 1, epoch, slotsOf(time.Minute, 2*time.Minute))

	airings := schedule.Between(epoch.Add(-3*time.Minute), epoch.Add(3*time.Minute))
	want := []int64{-2, -1, 0, 1}
	if len(airings) != len(want) {
		t.Fatalf("%d airings, want %d", len(airings), len(want))
	}
	for i, airing := range airings {
		if airing.Number != want[i] {
			t.Errorf("airing %d is number %d, want %d", i, airing.Number, want[i])
		}
	}
}

func TestSchedule_EmptyHasNoPieces(t *testing.T) {
	schedule := NewSchedule(channelmodel.OrderShuffle, 1, epoch, nil)

	if pieces := schedule.Pieces(step, epoch, epoch.Add(time.Hour)); len(pieces) != 0 {
		t.Errorf("an empty schedule has %d pieces", len(pieces))
	}
	if _, found := schedule.Piece(step, 0); found {
		t.Error("an empty schedule found a piece")
	}
}
