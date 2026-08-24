package scanner

import (
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func legacy(kind items.Kind, name string, year, index, parentIndex *int32, parent *uuid.UUID) *items.Item {
	record := &items.Item{
		ID:                uuid.New(),
		Kind:              kind,
		Name:              name,
		ProductionYear:    year,
		IndexNumber:       index,
		ParentIndexNumber: parentIndex,
		Key:               "/media/whatever/" + name,
	}
	record.ParentID = parent

	return record
}

func TestRekeyDerivesWhatTheScanWouldHaveWritten(t *testing.T) {
	series := legacy(itemmodal.KindSeries, "The Wire", ptr(int32(2002)), nil, nil, nil)
	season := legacy(itemmodal.KindSeason, "Season 1", nil, ptr(int32(1)), nil, &series.ID)
	episode := legacy(itemmodal.KindEpisode, "The Target", nil, ptr(int32(1)), ptr(int32(1)), &season.ID)
	movie := legacy(itemmodal.KindMovie, "The Matrix", ptr(int32(1999)), nil, nil, nil)

	byID := map[uuid.UUID]*items.Item{series.ID: series, season.ID: season, episode.ID: episode}

	tests := []struct {
		name string
		item *items.Item
		want string
	}{
		{"movie", movie, movieKey("The Matrix", ptr(int32(1999)))},
		{"series", series, seriesKey(titleSlug("The Wire", ptr(int32(2002))))},
		{"season", season, seasonKey(titleSlug("The Wire", ptr(int32(2002))), ptr(int32(1)))},
		{"episode", episode, episodeKey(titleSlug("The Wire", ptr(int32(2002))), ptr(int32(1)), ptr(int32(1)), "The Target")},
	}

	for _, test := range tests {
		got, ok := derivedKey(test.item, byID)
		if !ok {
			t.Errorf("%s derived no key", test.name)
			continue
		}
		if got != test.want {
			t.Errorf("%s = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestRekeyLeavesAnItemItCannotPlaceAlone(t *testing.T) {
	orphan := legacy(itemmodal.KindEpisode, "Stray", nil, ptr(int32(1)), ptr(int32(1)), nil)

	if _, ok := derivedKey(orphan, map[uuid.UUID]*items.Item{}); ok {
		t.Error("an episode with no series derived a key, which would collide under a shared slug")
	}
}
