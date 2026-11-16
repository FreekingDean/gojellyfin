package channels

import (
	"context"
	"encoding/binary"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

const (
	maxNumber    = 65535
	numberOffset = 6
	startOffset  = 8
)

var programPrefix = uuid.MustParse("c4a77e15-0000-0000-0000-000000000000")

func ProgramID(number int32, start time.Time) uuid.UUID {
	id := programPrefix
	binary.BigEndian.PutUint16(id[numberOffset:], uint16(number))
	binary.BigEndian.PutUint64(id[startOffset:], uint64(start.UnixMilli()))

	return id
}

func ParseProgram(id uuid.UUID) (int32, time.Time, bool) {
	if [numberOffset]byte(id[:numberOffset]) != [numberOffset]byte(programPrefix[:numberOffset]) {
		return 0, time.Time{}, false
	}

	number := int32(binary.BigEndian.Uint16(id[numberOffset:]))
	start := time.UnixMilli(int64(binary.BigEndian.Uint64(id[startOffset:]))).UTC()

	return number, start, number > 0
}

type Program struct {
	Channel *Channel
	Airing
}

func (p Program) ID() uuid.UUID {
	return ProgramID(p.Channel.Number, p.Start)
}

func (s *Service) Program(ctx context.Context, viewer items.Viewer, id uuid.UUID) (Program, error) {
	number, start, parsed := ParseProgram(id)
	if !parsed {
		return Program{}, ErrNotFound
	}

	channel, err := s.ByNumber(ctx, viewer, number)
	if err != nil {
		return Program{}, err
	}

	schedule, err := s.Schedule(ctx, channel)
	if err != nil {
		return Program{}, err
	}

	airing, found := schedule.At(start.Add(time.Millisecond / 2))
	if !found || airing.Start.UnixMilli() != start.UnixMilli() {
		return Program{}, ErrNotFound
	}

	return Program{Channel: channel, Airing: airing}, nil
}

type Guide struct {
	Viewer     items.Viewer
	ChannelIDs []uuid.UUID
	From       time.Time
	To         time.Time
}

func (s *Service) Programs(ctx context.Context, guide Guide) ([]Program, error) {
	listed, err := s.List(ctx, guide.Viewer)
	if err != nil {
		return nil, err
	}

	programs := []Program{}
	for _, channel := range listed {
		if len(guide.ChannelIDs) > 0 && !slices.Contains(guide.ChannelIDs, channel.ID) {
			continue
		}

		schedule, err := s.Schedule(ctx, channel)
		if err != nil {
			return nil, err
		}
		for _, airing := range schedule.Between(guide.From, guide.To) {
			programs = append(programs, Program{Channel: channel, Airing: airing})
		}
	}

	return programs, nil
}

func (s *Service) Now() time.Time {
	return s.now()
}
