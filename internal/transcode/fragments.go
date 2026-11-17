package transcode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
)

const maxHeaderBox = 16 << 20

var ErrNoHeader = errors.New("the segment carried no movie header")

type box struct {
	kind string
	body int64
}

func readBox(r io.Reader) (box, error) {
	var head [8]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return box{}, err
	}

	size := int64(binary.BigEndian.Uint32(head[:4]))
	kind := string(head[4:])
	switch size {
	case 0:
		return box{kind: kind, body: math.MaxInt64}, nil
	case 1:
		var large [8]byte
		if _, err := io.ReadFull(r, large[:]); err != nil {
			return box{}, err
		}
		extended := binary.BigEndian.Uint64(large[:])
		if extended < 16 || extended > math.MaxInt64 {
			return box{}, fmt.Errorf("a %q box claims %d bytes", kind, extended)
		}

		return box{kind: kind, body: int64(extended) - 16}, nil
	default:
		if size < 8 {
			return box{}, fmt.Errorf("a %q box claims %d bytes", kind, size)
		}

		return box{kind: kind, body: size - 8}, nil
	}
}

func (b box) read(r io.Reader) ([]byte, error) {
	if b.body > maxHeaderBox {
		return nil, fmt.Errorf("a %q box of %d bytes is too large to hold", b.kind, b.body)
	}

	body := make([]byte, b.body)
	_, err := io.ReadFull(r, body)

	return body, err
}

func (b box) encode(body []byte) []byte {
	return append(box{kind: b.kind, body: int64(len(body))}.head(), body...)
}

func children(body []byte) map[string][]byte {
	found := map[string][]byte{}
	for len(body) >= 8 {
		size := int(binary.BigEndian.Uint32(body[:4]))
		if size < 8 || size > len(body) {
			break
		}
		kind := string(body[4:8])
		if _, seen := found[kind]; !seen {
			found[kind] = body[8:size]
		}
		body = body[size:]
	}

	return found
}

func timescale(moov []byte) (uint32, error) {
	mdhd := children(children(children(moov)["trak"])["mdia"])["mdhd"]
	offset := 12
	if len(mdhd) > 0 && mdhd[0] == 1 {
		offset = 20
	}
	if len(mdhd) < offset+4 || binary.BigEndian.Uint32(mdhd[offset:offset+4]) == 0 {
		return 0, fmt.Errorf("the movie header names no timescale")
	}

	return binary.BigEndian.Uint32(mdhd[offset : offset+4]), nil
}

func decodeTime(moof []byte) (uint64, error) {
	tfdt := children(children(moof)["traf"])["tfdt"]
	switch {
	case len(tfdt) >= 12 && tfdt[0] == 1:
		return binary.BigEndian.Uint64(tfdt[4:12]), nil
	case len(tfdt) >= 8:
		return uint64(binary.BigEndian.Uint32(tfdt[4:8])), nil
	default:
		return 0, fmt.Errorf("a fragment carries no decode time")
	}
}

func presentationTime(moof []byte) (int64, error) {
	decoded, err := decodeTime(moof)
	if err != nil {
		return 0, err
	}

	return int64(decoded) + firstCompositionOffset(children(children(moof)["traf"])["trun"]), nil
}

func firstCompositionOffset(trun []byte) int64 {
	const (
		dataOffset       = 0x1
		firstSampleFlags = 0x4
		sampleDuration   = 0x100
		sampleSize       = 0x200
		sampleFlags      = 0x400
		sampleOffset     = 0x800
	)
	if len(trun) < 8 {
		return 0
	}
	flags := binary.BigEndian.Uint32(trun[:4]) & 0xffffff
	if flags&sampleOffset == 0 || binary.BigEndian.Uint32(trun[4:8]) == 0 {
		return 0
	}

	at := 8
	for _, present := range []uint32{dataOffset, firstSampleFlags, sampleDuration, sampleSize, sampleFlags} {
		if flags&present != 0 {
			at += 4
		}
	}
	if len(trun) < at+4 {
		return 0
	}
	offset := binary.BigEndian.Uint32(trun[at : at+4])
	if trun[0] == 0 {
		return int64(offset)
	}

	return int64(int32(offset))
}

func ticks(units uint64, scale uint32) int64 {
	product := new(big.Int).Mul(new(big.Int).SetUint64(units), big.NewInt(ticksPerSecond))

	return product.Div(product, big.NewInt(int64(scale))).Int64()
}

func presented(units int64, scale uint32) int64 {
	if units < 0 {
		return -ticks(uint64(-units), scale)
	}

	return ticks(uint64(units), scale)
}

func Header(w io.Writer, r io.Reader) error {
	for {
		current, err := readBox(r)
		if errors.Is(err, io.EOF) {
			return ErrNoHeader
		}
		if err != nil {
			return err
		}

		switch current.kind {
		case "ftyp", "moov":
			body, err := current.read(r)
			if err != nil {
				return err
			}
			if _, err := w.Write(current.encode(body)); err != nil {
				return err
			}
			if current.kind == "moov" {
				return nil
			}
		default:
			return ErrNoHeader
		}
	}
}

func Fragments(w io.Writer, r io.Reader, window Window) error {
	var scale uint32
	keep := false
	for {
		current, err := readBox(r)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		switch current.kind {
		case "moov":
			body, err := current.read(r)
			if err != nil {
				return err
			}
			if scale, err = timescale(body); err != nil {
				return err
			}
		case "moof":
			body, err := current.read(r)
			if err != nil {
				return err
			}
			if scale == 0 {
				return ErrNoHeader
			}
			units, err := presentationTime(body)
			if err != nil {
				return err
			}
			at := presented(units, scale)
			if at >= window.End-tolerance {
				return nil
			}
			keep = at >= window.Start-tolerance
			if keep {
				if _, err := w.Write(current.encode(body)); err != nil {
					return err
				}
			}
		case "mdat":
			if err := pass(w, r, current, keep); err != nil {
				return err
			}
		default:
			if err := pass(io.Discard, r, current, false); err != nil {
				return err
			}
		}
	}
}

func pass(w io.Writer, r io.Reader, current box, keep bool) error {
	if !keep {
		w = io.Discard
	} else if _, err := w.Write(current.head()); err != nil {
		return err
	}

	if current.body == math.MaxInt64 {
		_, err := io.Copy(w, r)

		return err
	}

	_, err := io.CopyN(w, r, current.body)

	return err
}

func (b box) head() []byte {
	switch {
	case b.body == math.MaxInt64:
		return append(binary.BigEndian.AppendUint32(nil, 0), b.kind...)
	case b.body+8 > math.MaxUint32:
		head := append(binary.BigEndian.AppendUint32(nil, 1), b.kind...)

		return binary.BigEndian.AppendUint64(head, uint64(b.body+16))
	default:
		return append(binary.BigEndian.AppendUint32(nil, uint32(b.body+8)), b.kind...)
	}
}
