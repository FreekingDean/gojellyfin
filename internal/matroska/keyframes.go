package matroska

import (
	"errors"
	"io"
	"slices"
	"time"
)

const (
	idEBML           = 0x1A45DFA3
	idDocType        = 0x4282
	idSegment        = 0x18538067
	idSeekHead       = 0x114D9B74
	idSeek           = 0x4DBB
	idSeekID         = 0x53AB
	idSeekPosition   = 0x53AC
	idInfo           = 0x1549A966
	idTimestampScale = 0x2AD7B1
	idTracks         = 0x1654AE6B
	idTrackEntry     = 0xAE
	idTrackNumber    = 0xD7
	idTrackType      = 0x83
	idCues           = 0x1C53BB6B
	idCuePoint       = 0xBB
	idCueTime        = 0xB3
	idCueTrackPos    = 0xB7
	idCueTrack       = 0xF7
	idCluster        = 0x1F43B675

	videoTrack     = 1
	defaultScaleNs = 1_000_000
)

var ErrNoIndex = errors.New("matroska: the file carries no keyframe index")

type segment struct {
	r       *reader
	start   int64
	scale   uint64
	video   uint64
	cues    int64
	seeks   []int64
	visited map[int64]bool
}

func Keyframes(file io.ReadSeeker) ([]time.Duration, error) {
	r := newReader(file)
	if err := matroskaHeader(r); err != nil {
		return nil, err
	}
	top, err := r.header()
	if err != nil {
		return nil, err
	}
	if top.id != idSegment {
		return nil, ErrNoIndex
	}

	s := &segment{r: r, start: top.data, scale: defaultScaleNs, cues: -1, visited: map[int64]bool{}}
	if err := s.scan(top); err != nil {
		return nil, err
	}

	return s.keyframes()
}

func matroskaHeader(r *reader) error {
	ebml, err := r.header()
	if err != nil || ebml.id != idEBML {
		return ErrNoIndex
	}
	docType := ""
	err = r.children(ebml, func(child header) error {
		if child.id != idDocType {
			return nil
		}
		raw, err := r.payload(child)
		docType = string(raw)

		return err
	})
	if err != nil {
		return err
	}
	if docType != "matroska" && docType != "webm" {
		return ErrNoIndex
	}

	return r.seek(ebml.end())
}

func (s *segment) scan(top header) error {
	errStop := errors.New("stop")
	err := s.r.children(top, func(child header) error {
		if child.id == idCluster {
			return errStop
		}

		return s.visit(child)
	})
	if err != nil && !errors.Is(err, errStop) {
		return err
	}

	return nil
}

func (s *segment) visit(child header) error {
	if s.visited[child.offset] {
		return nil
	}
	s.visited[child.offset] = true

	switch child.id {
	case idSeekHead:
		return s.r.children(child, s.seek)
	case idInfo:
		return s.r.children(child, s.info)
	case idTracks:
		return s.r.children(child, s.track)
	case idCues:
		s.cues = child.offset
	}

	return nil
}

func (s *segment) seek(entry header) error {
	if entry.id != idSeek {
		return nil
	}
	var id, position uint64
	err := s.r.children(entry, func(field header) error {
		var err error
		switch field.id {
		case idSeekID:
			var raw []byte
			raw, err = s.r.payload(field)
			for _, b := range raw {
				id = id<<8 | uint64(b)
			}
		case idSeekPosition:
			position, err = s.r.uint(field)
		}

		return err
	})
	if err != nil {
		return err
	}
	switch id {
	case idCues:
		s.cues = s.start + int64(position)
	case idSeekHead, idInfo, idTracks:
		s.seeks = append(s.seeks, s.start+int64(position))
	}

	return nil
}

func (s *segment) info(field header) error {
	if field.id != idTimestampScale {
		return nil
	}
	scale, err := s.r.uint(field)
	if err == nil && scale > 0 {
		s.scale = scale
	}

	return err
}

func (s *segment) track(entry header) error {
	if entry.id != idTrackEntry || s.video != 0 {
		return nil
	}
	var number, kind uint64
	err := s.r.children(entry, func(field header) error {
		var err error
		switch field.id {
		case idTrackNumber:
			number, err = s.r.uint(field)
		case idTrackType:
			kind, err = s.r.uint(field)
		}

		return err
	})
	if kind == videoTrack {
		s.video = number
	}

	return err
}

func (s *segment) follow() error {
	for len(s.seeks) > 0 {
		offset := s.seeks[0]
		s.seeks = s.seeks[1:]
		if s.visited[offset] {
			continue
		}
		if err := s.r.seek(offset); err != nil {
			return err
		}
		element, err := s.r.header()
		if err != nil {
			return err
		}
		if err := s.visit(element); err != nil {
			return err
		}
	}

	return nil
}

func (s *segment) keyframes() ([]time.Duration, error) {
	if err := s.follow(); err != nil {
		return nil, err
	}
	if s.video == 0 || s.cues < 0 {
		return nil, ErrNoIndex
	}
	if err := s.r.seek(s.cues); err != nil {
		return nil, err
	}
	cues, err := s.r.header()
	if err != nil {
		return nil, err
	}
	if cues.id != idCues || !cues.sized() {
		return nil, ErrNoIndex
	}

	times := []time.Duration{}
	err = s.r.children(cues, func(point header) error {
		if point.id != idCuePoint {
			return nil
		}
		at, video, err := s.point(point)
		if video {
			times = append(times, time.Duration(at*s.scale))
		}

		return err
	})
	if err != nil {
		return nil, err
	}
	if len(times) == 0 {
		return nil, ErrNoIndex
	}
	slices.Sort(times)

	return slices.Compact(times), nil
}

func (s *segment) point(point header) (uint64, bool, error) {
	var at uint64
	video := false
	err := s.r.children(point, func(field header) error {
		switch field.id {
		case idCueTime:
			var err error
			at, err = s.r.uint(field)

			return err
		case idCueTrackPos:
			return s.r.children(field, func(position header) error {
				if position.id != idCueTrack {
					return nil
				}
				track, err := s.r.uint(position)
				video = video || track == s.video

				return err
			})
		}

		return nil
	})

	return at, video, err
}
