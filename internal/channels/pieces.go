package channels

import "time"

type Piece struct {
	Airing
	Index    int
	Sequence int64
	Begins   time.Time
	Ends     time.Time
}

func (p Piece) Length() time.Duration {
	return p.Ends.Sub(p.Begins)
}

func Count(length, step time.Duration) int {
	if length <= step {
		return 1
	}

	return int((length + step - 1) / step)
}

func (s Schedule) Pieces(step time.Duration, from, to time.Time) []Piece {
	if step <= 0 {
		return nil
	}

	pieces := []Piece{}
	for _, airing := range s.Between(from, to) {
		first := s.sequence(step, airing)
		for index := range Count(airing.Length, step) {
			piece := cut(airing, step, index, first+int64(index))
			if piece.Ends.After(from) && piece.Begins.Before(to) {
				pieces = append(pieces, piece)
			}
		}
	}

	return pieces
}

func (s Schedule) Piece(step time.Duration, sequence int64) (Piece, bool) {
	perCycle := s.perCycle(step)
	if perCycle == 0 {
		return Piece{}, false
	}

	cycle := sequence / perCycle
	remaining := sequence % perCycle
	if remaining < 0 {
		cycle--
		remaining += perCycle
	}

	order, offsets := s.ordered(cycle)
	start := s.epoch.Add(time.Duration(cycle) * s.cycle)
	for i, slot := range order {
		count := int64(Count(slot.Length, step))
		if remaining < count {
			airing := s.airing(cycle, i, slot, start.Add(offsets[i]))

			return cut(airing, step, int(remaining), sequence), true
		}
		remaining -= count
	}

	return Piece{}, false
}

func (s Schedule) perCycle(step time.Duration) int64 {
	if step <= 0 {
		return 0
	}

	total := int64(0)
	for _, slot := range s.slots {
		total += int64(Count(slot.Length, step))
	}

	return total
}

func (s Schedule) sequence(step time.Duration, airing Airing) int64 {
	slots := int64(len(s.slots))
	cycle := airing.Number / slots
	position := airing.Number % slots
	if position < 0 {
		cycle--
		position += slots
	}

	order, _ := s.ordered(cycle)
	sequence := cycle * s.perCycle(step)
	for _, slot := range order[:position] {
		sequence += int64(Count(slot.Length, step))
	}

	return sequence
}

func cut(airing Airing, step time.Duration, index int, sequence int64) Piece {
	begins := airing.Start.Add(time.Duration(index) * step)

	return Piece{
		Airing:   airing,
		Index:    index,
		Sequence: sequence,
		Begins:   begins,
		Ends:     minTime(begins.Add(step), airing.End),
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}

	return b
}
