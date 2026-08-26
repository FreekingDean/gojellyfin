package items

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_ItemsNeedingMetadata(t *testing.T) {
	wanted := []Kind{itemmodal.KindMovie, itemmodal.KindSeries, itemmodal.KindSeason, itemmodal.KindEpisode}

	t.Run("hands the kinds back in the order they were asked for", func(t *testing.T) {
		fixed := newFixture(t)

		kinds := map[uuid.UUID]Kind{}
		for _, kind := range []Kind{itemmodal.KindEpisode, itemmodal.KindSeason, itemmodal.KindSeries, itemmodal.KindMovie} {
			for copy := range 3 {
				id := fixed.add(t, seed{kind: kind, name: fmt.Sprintf("%s %d", kind, copy)})
				kinds[id] = kind
			}
		}

		pending, err := fixed.service.ItemsNeedingMetadata(context.Background(), wanted, false, uuid.Nil)
		if err != nil {
			t.Fatalf("failed to select the items needing metadata: %v", err)
		}
		if len(pending) != len(kinds) {
			t.Fatalf("selected %d items, want %d", len(pending), len(kinds))
		}

		ranks := make([]int, 0, len(pending))
		for _, id := range pending {
			ranks = append(ranks, slices.Index(wanted, kinds[id]))
		}
		if !slices.IsSorted(ranks) {
			t.Errorf("kind order = %v, want parents before children", ranks)
		}
	})

	t.Run("leaves an identified item out unless forced", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.add(t, seed{kind: itemmodal.KindMovie, name: "The Matrix"})

		if _, err := fixed.service.UpdateMetadata(context.Background(), movie, Metadata{
			ProviderIds: &map[string]string{"Stub": "603"},
		}); err != nil {
			t.Fatalf("failed to identify the movie: %v", err)
		}

		pending, err := fixed.service.ItemsNeedingMetadata(context.Background(), wanted, false, uuid.Nil)
		if err != nil {
			t.Fatalf("failed to select the items needing metadata: %v", err)
		}
		if slices.Contains(pending, movie) {
			t.Error("the identified movie was selected, want it left out")
		}

		forced, err := fixed.service.ItemsNeedingMetadata(context.Background(), wanted, true, uuid.Nil)
		if err != nil {
			t.Fatalf("failed to select the forced items: %v", err)
		}
		if !slices.Contains(forced, movie) {
			t.Error("the identified movie was not selected when forced, want it back")
		}
	})

	t.Run("takes a series scope down to its episodes", func(t *testing.T) {
		fixed := newFixture(t)
		series := fixed.add(t, seed{kind: itemmodal.KindSeries, name: "Breaking Bad"})
		season := fixed.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series})
		episode := fixed.add(t, seed{kind: itemmodal.KindEpisode, name: "Pilot", parentID: &season})
		elsewhere := fixed.add(t, seed{kind: itemmodal.KindMovie, name: "The Matrix"})

		pending, err := fixed.service.ItemsNeedingMetadata(context.Background(), wanted, false, series)
		if err != nil {
			t.Fatalf("failed to select the scoped items: %v", err)
		}

		for _, id := range []uuid.UUID{series, season, episode} {
			if !slices.Contains(pending, id) {
				t.Errorf("%v was not selected, want the whole series in scope", id)
			}
		}
		if slices.Contains(pending, elsewhere) {
			t.Error("an unrelated item was selected, want the scope to stop at the series")
		}
	})
}
