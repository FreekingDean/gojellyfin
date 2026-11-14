package tmdb

import (
	"context"
	"testing"

	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestClient_Search(t *testing.T) {
	ctx := context.Background()

	t.Run("a movie carries what a card needs", func(t *testing.T) {
		titles, err := stub(t).Search(ctx, itemmodel.KindMovie, "the matrix")
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if len(titles) != 2 {
			t.Fatalf("titles = %d, want 2", len(titles))
		}

		matrix := titles[0]
		if *matrix.Name != "The Matrix" || *matrix.ProductionYear != 1999 {
			t.Errorf("matrix = %q, %d", *matrix.Name, *matrix.ProductionYear)
		}
		if got := (*matrix.ProviderIds)[providerTmdb]; got != "603" {
			t.Errorf("tmdb id = %q, want 603", got)
		}
		if got := artworkURL(t, matrix.Images, imagemodel.KindPrimary); got != "https://image.tmdb.org/t/p/w780/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg" {
			t.Errorf("poster = %q", got)
		}
	})

	t.Run("a series is named and dated by the fields a series uses", func(t *testing.T) {
		titles, err := stub(t).Search(ctx, itemmodel.KindSeries, "breaking")
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		if len(titles) != 1 || *titles[0].Name != "Breaking Bad" || *titles[0].ProductionYear != 2008 {
			t.Fatalf("titles = %+v", titles)
		}
	})

	t.Run("a kind nothing is searched for as is refused before a request", func(t *testing.T) {
		stubbing := newStub(t)
		if _, err := stubbing.client.Search(ctx, itemmodel.KindEpisode, "pilot"); err == nil {
			t.Error("episodes were searched for")
		}
		if asked := stubbing.requests(); len(asked) != 0 {
			t.Errorf("asked = %v, want nothing", asked)
		}
	})
}
