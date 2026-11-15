package items

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_SaveImage(t *testing.T) {
	t.Run("replaces the poster it wrote before", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		movie := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Dune"})

		for _, tag := range []string{"abc", "def"} {
			poster := Image{
				Kind: imagemodel.KindPrimary,
				URL:  "https://image.tmdb.org/t/p/w780/" + tag + ".jpg",
				Tag:  tag,
			}
			if err := fixture.service.SaveImage(ctx, movie, poster); err != nil {
				t.Fatalf("failed to save %q: %v", tag, err)
			}
		}

		record, err := fixture.service.Image(ctx, movie, imagemodel.KindPrimary, 0)
		if err != nil {
			t.Fatalf("failed to read the image back: %v", err)
		}
		if record.URL != "https://image.tmdb.org/t/p/w780/def.jpg" {
			t.Errorf("url = %q, want the newer poster", record.URL)
		}
		if record.Tag != "def" {
			t.Errorf("tag = %q, want a changed poster to bust the client's cache", record.Tag)
		}
	})
}

func TestService_Posters(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	poster := func(itemID uuid.UUID, tag string) {
		t.Helper()
		if err := fixture.service.SaveImage(ctx, itemID, Image{Kind: imagemodel.KindPrimary, URL: "https://cdn.invalid/" + tag, Tag: tag}); err != nil {
			t.Fatalf("failed to save %q: %v", tag, err)
		}
	}

	show := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Show"})
	season := fixture.add(t, seed{kind: itemmodel.KindSeason, name: "Show S1", parentID: &show, index: number(1)})
	first := fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Show E1", parentID: &season, index: number(1), parentIndex: number(1)})
	second := fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Show E2", parentID: &season, index: number(2), parentIndex: number(1)})
	film := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Film"})
	bare := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Bare"})
	poster(show, "show")
	poster(first, "still")
	poster(film, "film")

	records, err := fixture.service.ItemsByIDs(ctx, Everyone, []uuid.UUID{first, bare, second, film})
	if err != nil {
		t.Fatalf("failed to read the items: %v", err)
	}
	byID := map[uuid.UUID]*Item{}
	for _, record := range records {
		byID[record.ID] = record
	}
	ordered := []*Item{byID[first], byID[bare], byID[second], byID[film]}

	tests := []struct {
		name  string
		limit int
		want  []string
	}{
		{name: "an episode shows its series' poster once, and a title without one is skipped", limit: 10, want: []string{"show", "film"}},
		{name: "the limit counts posters", limit: 1, want: []string{"show"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			posters, err := fixture.service.Posters(ctx, ordered, test.limit)
			if err != nil {
				t.Fatalf("failed to read the posters: %v", err)
			}

			got := make([]string, 0, len(posters))
			for _, entry := range posters {
				got = append(got, entry.Tag)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("posters = %v, want %v", got, test.want)
			}
		})
	}
}
