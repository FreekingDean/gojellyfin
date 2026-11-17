package matroska

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

var errMalformed = errors.New("matroska: malformed element")

const unknownSize = math.MaxUint64

type header struct {
	id     uint32
	size   uint64
	offset int64
	data   int64
}

func (h header) end() int64 {
	return h.data + int64(h.size)
}

func (h header) sized() bool {
	return h.size != unknownSize
}

type reader struct {
	file io.ReadSeeker
	buf  *bufio.Reader
	at   int64
}

func newReader(file io.ReadSeeker) *reader {
	return &reader{file: file, buf: bufio.NewReaderSize(file, 64*1024)}
}

func (r *reader) seek(offset int64) error {
	if offset == r.at {
		return nil
	}
	if _, err := r.file.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	r.buf.Reset(r.file)
	r.at = offset

	return nil
}

func (r *reader) byte() (byte, error) {
	b, err := r.buf.ReadByte()
	if err == nil {
		r.at++
	}

	return b, err
}

func (r *reader) vint(keepMarker bool) (uint64, int, error) {
	first, err := r.byte()
	if err != nil {
		return 0, 0, err
	}
	width := 1
	for mask := byte(0x80); width <= 8 && first&mask == 0; mask >>= 1 {
		width++
	}
	if width > 8 {
		return 0, 0, errMalformed
	}
	value := uint64(first)
	if !keepMarker {
		value &= uint64(0xFF >> width)
	}
	allOnes := value == uint64(0xFF>>width)
	for range width - 1 {
		next, err := r.byte()
		if err != nil {
			return 0, 0, err
		}
		value = value<<8 | uint64(next)
		allOnes = allOnes && next == 0xFF
	}
	if !keepMarker && allOnes {
		return unknownSize, width, nil
	}

	return value, width, nil
}

func (r *reader) header() (header, error) {
	offset := r.at
	id, width, err := r.vint(true)
	if err != nil {
		return header{}, err
	}
	if width > 4 {
		return header{}, errMalformed
	}
	size, _, err := r.vint(false)
	if err != nil {
		return header{}, err
	}

	return header{id: uint32(id), size: size, offset: offset, data: r.at}, nil
}

func (r *reader) payload(h header) ([]byte, error) {
	if !h.sized() || h.size > 1<<20 {
		return nil, errMalformed
	}
	out := make([]byte, h.size)
	if _, err := io.ReadFull(r.buf, out); err != nil {
		return nil, err
	}
	r.at += int64(h.size)

	return out, nil
}

func (r *reader) uint(h header) (uint64, error) {
	raw, err := r.payload(h)
	if err != nil || len(raw) > 8 {
		return 0, errors.Join(err, errMalformedIf(len(raw) > 8))
	}
	padded := make([]byte, 8)
	copy(padded[8-len(raw):], raw)

	return binary.BigEndian.Uint64(padded), nil
}

func errMalformedIf(bad bool) error {
	if bad {
		return errMalformed
	}

	return nil
}

func (r *reader) children(parent header, visit func(header) error) error {
	for r.at < parent.end() || !parent.sized() {
		child, err := r.header()
		if errors.Is(err, io.EOF) && !parent.sized() {
			return nil
		}
		if err != nil {
			return err
		}
		if err := visit(child); err != nil {
			return err
		}
		if !child.sized() {
			return nil
		}
		if err := r.seek(child.end()); err != nil {
			return err
		}
	}

	return nil
}
