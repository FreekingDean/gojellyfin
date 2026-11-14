package discover

import (
	"encoding/binary"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const (
	kindByte = 11
	idOffset = 12
)

var prefix = uuid.MustParse("d15c07e2-71b1-4e5a-9f00-000000000000")

var kindCodes = map[items.Kind]byte{
	itemmodel.KindMovie:  1,
	itemmodel.KindSeries: 2,
}

func ID(kind items.Kind, tmdbID uint32) uuid.UUID {
	id := prefix
	id[kindByte] = kindCodes[kind]
	binary.BigEndian.PutUint32(id[idOffset:], tmdbID)

	return id
}

func Parse(id uuid.UUID) (items.Kind, uint32, bool) {
	for index := range kindByte {
		if id[index] != prefix[index] {
			return "", 0, false
		}
	}

	tmdbID := binary.BigEndian.Uint32(id[idOffset:])
	for kind, code := range kindCodes {
		if code == id[kindByte] && tmdbID > 0 {
			return kind, tmdbID, true
		}
	}

	return "", 0, false
}

func Key(kind items.Kind, tmdbID uint32) string {
	if kind == itemmodel.KindSeries {
		return items.SeriesKey(int(tmdbID))
	}

	return items.MovieKey(int(tmdbID))
}
