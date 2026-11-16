package items

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (f *fixture) rate(t *testing.T, id uuid.UUID, level int) {
	t.Helper()

	if err := f.client.Item.UpdateOneID(id).SetParentalRating(level).Exec(context.Background()); err != nil {
		t.Fatalf("failed to rate the item: %v", err)
	}
}

func TestQueryItems_ParentalRating(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	family := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Family"})
	fixture.rate(t, family, 0)
	teen := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Teen"})
	fixture.rate(t, teen, 13)
	adult := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Adult"})
	fixture.rate(t, adult, 17)
	fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Unrated"})

	show := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Late Show"})
	fixture.rate(t, show, 17)
	season := fixture.add(t, seed{kind: itemmodel.KindSeason, name: "Late Show Season", parentID: &show})
	fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Late Show Episode", parentID: &season})

	cartoon := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Cartoon"})
	fixture.rate(t, cartoon, 0)
	cartoonSeason := fixture.add(t, seed{kind: itemmodel.KindSeason, name: "Cartoon Season", parentID: &cartoon})
	explicit := fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Cartoon Special", parentID: &cartoonSeason})
	fixture.rate(t, explicit, 17)

	fixture.add(t, seed{kind: itemmodel.KindMusicArtist, name: "Band"})

	thirteen := 13
	everything := []string{
		"Adult", "Band", "Cartoon", "Cartoon Season", "Cartoon Special", "Family",
		"Late Show", "Late Show Episode", "Late Show Season", "Teen", "Unrated",
	}
	for _, test := range []struct {
		name   string
		viewer Viewer
		want   []string
	}{
		{
			name:   "no limit sees everything",
			viewer: Everyone,
			want:   everything,
		},
		{
			name:   "a limit hides what is rated above it, and what inherits a rating above it",
			viewer: Viewer{All: true, MaxRating: &thirteen},
			want:   []string{"Band", "Cartoon", "Cartoon Season", "Family", "Teen", "Unrated"},
		},
		{
			name:   "blocking unrated movies leaves unrated music",
			viewer: Viewer{All: true, MaxRating: &thirteen, BlockUnrated: []string{"Movie"}},
			want:   []string{"Band", "Cartoon", "Cartoon Season", "Family", "Teen"},
		},
		{
			name:   "blocking unrated music without a limit hides only the music",
			viewer: Viewer{All: true, BlockUnrated: []string{"Music"}},
			want: []string{
				"Adult", "Cartoon", "Cartoon Season", "Cartoon Special", "Family",
				"Late Show", "Late Show Episode", "Late Show Season", "Teen", "Unrated",
			},
		},
		{
			name:   "a limit applies inside the libraries a user may see",
			viewer: Viewer{Libraries: []uuid.UUID{fixture.libraryID}, MaxRating: &thirteen},
			want:   []string{"Band", "Cartoon", "Cartoon Season", "Family", "Teen", "Unrated"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			records, _, err := fixture.service.QueryItems(ctx, ItemQuery{Viewer: test.viewer, LibraryID: &fixture.libraryID})
			if err != nil {
				t.Fatalf("failed to query items: %v", err)
			}

			found := make([]string, 0, len(records))
			for _, record := range records {
				found = append(found, record.Name)
			}
			slices.Sort(found)
			if !slices.Equal(found, test.want) {
				t.Errorf("items = %v, want %v", found, test.want)
			}
		})
	}

	if _, err := fixture.service.ItemByID(ctx, Viewer{All: true, MaxRating: &thirteen}, adult); err == nil {
		t.Error("ItemByID answered a title rated above the limit")
	}
}
