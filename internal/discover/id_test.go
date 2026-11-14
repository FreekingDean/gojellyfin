package discover

import (
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestID(t *testing.T) {
	tests := []struct {
		name    string
		kind    items.Kind
		tmdbID  uint32
		wantKey string
	}{
		{name: "a movie", kind: itemmodel.KindMovie, tmdbID: 603, wantKey: "movie:tmdb:603"},
		{name: "a series", kind: itemmodel.KindSeries, tmdbID: 1396, wantKey: "series:tmdb:1396"},
		{name: "an id past sixteen bits", kind: itemmodel.KindMovie, tmdbID: 1234567, wantKey: "movie:tmdb:1234567"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kind, tmdbID, ok := Parse(ID(test.kind, test.tmdbID))
			if !ok || kind != test.kind || tmdbID != test.tmdbID {
				t.Errorf("Parse(ID(%s, %d)) = %s, %d, %t", test.kind, test.tmdbID, kind, tmdbID, ok)
			}
			if got := Key(kind, tmdbID); got != test.wantKey {
				t.Errorf("key = %q, want %q", got, test.wantKey)
			}
		})
	}

	if ID(itemmodel.KindMovie, 603) == ID(itemmodel.KindSeries, 603) {
		t.Error("a movie and a series sharing a TMDB id share an item id")
	}
}

func TestParse_RefusesWhatItDidNotWrite(t *testing.T) {
	zeroed := ID(itemmodel.KindMovie, 603)
	zeroed[kindByte] = 0

	unnumbered := prefix
	unnumbered[kindByte] = kindCodes[itemmodel.KindMovie]

	for name, id := range map[string]uuid.UUID{
		"a random id":           uuid.New(),
		"the nil id":            uuid.Nil,
		"a kind nothing writes": zeroed,
		"no TMDB id":            unnumbered,
	} {
		if _, _, ok := Parse(id); ok {
			t.Errorf("%s parsed as an unowned title", name)
		}
	}
}
