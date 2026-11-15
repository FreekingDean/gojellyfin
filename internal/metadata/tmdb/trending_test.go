package tmdb

import (
	"context"
	"errors"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/items"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestClient_Trending(t *testing.T) {
	ctx := context.Background()

	t.Run("movies carry what a card needs", func(t *testing.T) {
		titles, err := stub(t).Trending(ctx, itemmodel.KindMovie, 1)
		if err != nil {
			t.Fatalf("failed to read what is trending: %v", err)
		}
		if len(titles) != 2 {
			t.Fatalf("titles = %d, want 2", len(titles))
		}

		matrix := titles[0]
		if *matrix.Name != "The Matrix" || *matrix.ProductionYear != 1999 || *matrix.CommunityRating < 8 {
			t.Errorf("matrix = %q, %d, %v", *matrix.Name, *matrix.ProductionYear, *matrix.CommunityRating)
		}
		if got := (*matrix.ProviderIds)[providerTmdb]; got != "603" {
			t.Errorf("tmdb id = %q, want 603", got)
		}
		if got := artworkURL(t, matrix.Images, imagemodel.KindPrimary); got != "https://image.tmdb.org/t/p/w780/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg" {
			t.Errorf("poster = %q", got)
		}
		if got := artworkURL(t, matrix.Images, imagemodel.KindBackdrop); got != "https://image.tmdb.org/t/p/w1280/ByDf0zjLSumz1MP1cDEo2JmHkrn.jpg" {
			t.Errorf("backdrop = %q", got)
		}

		undated := titles[1]
		if undated.PremiereDate != nil || undated.ProductionYear != nil || undated.Images != nil {
			t.Errorf("a title with no date or artwork = %+v", undated)
		}
	})

	t.Run("a series is named and dated by the fields a series uses", func(t *testing.T) {
		titles, err := stub(t).Trending(ctx, itemmodel.KindSeries, 1)
		if err != nil {
			t.Fatalf("failed to read what is trending: %v", err)
		}
		if len(titles) != 1 || *titles[0].Name != "Breaking Bad" || *titles[0].ProductionYear != 2008 {
			t.Fatalf("titles = %+v", titles)
		}
	})

	t.Run("a kind nothing trends as is refused before a request", func(t *testing.T) {
		stubbing := newStub(t)
		if _, err := stubbing.client.Trending(ctx, itemmodel.KindEpisode, 1); err == nil {
			t.Error("episodes trended")
		}
		if asked := stubbing.requests(); len(asked) != 0 {
			t.Errorf("asked = %v, want nothing", asked)
		}
	})

	t.Run("without a key there is nothing to ask", func(t *testing.T) {
		client, err := newClient("", "")
		if err != nil {
			t.Fatalf("failed to build the client: %v", err)
		}
		if _, err := client.Trending(ctx, items.Kind(itemmodel.KindMovie), 1); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("err = %v, want ErrNotConfigured", err)
		}
	})
}
