package shelves

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type fixture struct {
	client    *store.Client
	catalogue *items.Service
	libraryID uuid.UUID
	sourceID  uuid.UUID
	userID    uuid.UUID
	prefix    string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString()

	library, err := client.Library.Create().SetName(prefix).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}
	downloader, err := client.Source.Create().
		SetName(prefix).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindRadarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	user, err := client.User.Create().SetName(prefix).SetUsername(prefix).SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.Item.Delete().Where(itemmodel.KeyHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.User.DeleteOne(user).Exec(ctx); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
		if err := client.Source.DeleteOne(downloader).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{
		client:    client,
		catalogue: items.New(client),
		libraryID: library.ID,
		sourceID:  downloader.ID,
		userID:    user.ID,
		prefix:    prefix,
	}
}

type film struct {
	name     string
	rating   float64
	minutes  int64
	genre    string
	released time.Time
}

func (f *fixture) movie(t *testing.T, movie film) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	record, err := f.client.Item.Create().
		SetKind(itemmodel.KindMovie).
		SetName(movie.name).
		SetSortName(strings.ToLower(movie.name)).
		SetKey(f.prefix + ":" + movie.name).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create %q: %v", movie.name, err)
	}
	if err := f.catalogue.SaveMembership(ctx, f.libraryID, f.sourceID, []uuid.UUID{record.ID}); err != nil {
		t.Fatalf("failed to place %q in the library: %v", movie.name, err)
	}

	runtime := movie.minutes * ticksPerMinute
	if _, err := f.catalogue.UpdateMetadata(ctx, record.ID, items.Metadata{
		CommunityRating: &movie.rating,
		RunTimeTicks:    &runtime,
		PremiereDate:    &movie.released,
		Genres:          &[]string{movie.genre},
	}); err != nil {
		t.Fatalf("failed to describe %q: %v", movie.name, err)
	}

	return record.ID
}

func (f *fixture) watched(t *testing.T, itemID uuid.UUID, at time.Time) {
	t.Helper()

	if err := f.catalogue.SaveUserItemDatum(context.Background(), &items.Datum{
		UserID:       f.userID,
		ItemID:       itemID,
		Played:       true,
		PlayCount:    1,
		LastPlayedAt: &at,
	}); err != nil {
		t.Fatalf("failed to mark the item watched: %v", err)
	}
}

func TestService_Shelves(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	october := time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC)
	year := func(value int) time.Time { return time.Date(value, time.June, 1, 0, 0, 0, 0, time.UTC) }

	fixture.movie(t, film{name: "Gem", rating: 8, minutes: 120, genre: "Drama", released: year(2020)})
	fixture.movie(t, film{name: "Dud", rating: 5, minutes: 90, genre: "Horror", released: year(2024)})
	fixture.movie(t, film{name: "Quick", rating: 7, minutes: 85, genre: "Comedy", released: year(2023)})
	fixture.movie(t, film{name: "Future", rating: 9, minutes: 90, genre: "Comedy", released: year(2030)})
	seen := fixture.movie(t, film{name: "Seen", rating: 9, minutes: 95, genre: "Drama", released: year(2010)})
	older := fixture.movie(t, film{name: "Older", rating: 6, minutes: 95, genre: "Horror", released: year(2005)})
	fixture.watched(t, older, october.AddDate(0, 0, -10))
	fixture.watched(t, seen, october.AddDate(0, 0, -1))

	viewer := items.Viewer{Libraries: []uuid.UUID{fixture.libraryID}}

	tests := []struct {
		slug     string
		now      time.Time
		wantName string
		want     []string
		absent   bool
	}{
		{slug: "new-releases", now: october, wantName: "New Releases", want: []string{"Dud", "Quick", "Gem", "Seen", "Older"}},
		{slug: "hidden-gems", now: october, wantName: "Hidden Gems", want: []string{"Future", "Gem"}},
		{slug: "short-on-time", now: october, wantName: "Short on Time", want: []string{"Future", "Quick", "Dud"}},
		{slug: "spooky-season", now: october, wantName: "Spooky Season", want: []string{"Older", "Dud"}},
		{slug: "spooky-season", now: october.AddDate(0, 1, 0), absent: true},
		{slug: "because-you-watched", now: october, wantName: "Because You Watched Seen", want: []string{"Gem"}},
	}

	for _, test := range tests {
		t.Run(test.slug+" "+test.now.Month().String(), func(t *testing.T) {
			service, err := newService(fixture.catalogue, []string{test.slug}, func() time.Time { return test.now })
			if err != nil {
				t.Fatalf("failed to build the shelves: %v", err)
			}

			listed, err := service.Shelves(ctx, viewer, fixture.userID)
			if err != nil {
				t.Fatalf("failed to list the shelves: %v", err)
			}
			if test.absent {
				if len(listed) != 0 {
					t.Errorf("shelves = %v, want none", listed)
				}
				return
			}
			if len(listed) != 1 || listed[0].Name != test.wantName || listed[0].ID != ID(test.slug) {
				t.Fatalf("shelves = %+v, want one named %q", listed, test.wantName)
			}

			latest, err := service.Latest(ctx, listed[0], 10, true)
			if err != nil {
				t.Fatalf("failed to read the shelf: %v", err)
			}

			got := make([]string, 0, len(latest))
			for _, entry := range latest {
				got = append(got, entry.Item.Name)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("items = %v, want %v", got, test.want)
			}
		})
	}
}

func TestService_GenreSpotlight(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	fixture.movie(t, film{name: "Gem", rating: 8, minutes: 120, genre: "Drama", released: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)})
	fixture.movie(t, film{name: "Quick", rating: 7, minutes: 85, genre: "Comedy", released: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)})

	viewer := items.Viewer{Libraries: []uuid.UUID{fixture.libraryID}}
	seen := map[string]bool{}
	for week := range 2 {
		now := time.Date(2026, time.January, 5+7*week, 0, 0, 0, 0, time.UTC)
		service, err := newService(fixture.catalogue, []string{"genre-spotlight"}, func() time.Time { return now })
		if err != nil {
			t.Fatalf("failed to build the shelves: %v", err)
		}

		listed, err := service.Shelves(ctx, viewer, fixture.userID)
		if err != nil || len(listed) != 1 {
			t.Fatalf("shelves = %v, %v, want one", listed, err)
		}

		latest, err := service.Latest(ctx, listed[0], 10, true)
		if err != nil || len(latest) != 1 {
			t.Fatalf("items = %v, %v, want one", latest, err)
		}
		if want := "Spotlight: " + map[string]string{"Gem": "Drama", "Quick": "Comedy"}[latest[0].Item.Name]; listed[0].Name != want {
			t.Errorf("name = %q, want %q", listed[0].Name, want)
		}
		seen[listed[0].Name] = true
	}

	if len(seen) != 2 {
		t.Errorf("spotlights over two weeks = %v, want both genres", seen)
	}
}

func TestNew_RefusesAnUnknownShelf(t *testing.T) {
	if _, err := newService(nil, []string{"recently-aired", "nonsense"}, time.Now); err == nil {
		t.Error("an unknown shelf was accepted")
	}

	service, err := newService(nil, []string{"none"}, time.Now)
	if err != nil || len(service.enabled) != 0 {
		t.Errorf("none = %v, %v, want no shelves", service, err)
	}

	service, err = newService(nil, nil, time.Now)
	if err != nil || len(service.enabled) != len(Defaults) {
		t.Errorf("unset = %v, %v, want the defaults", service, err)
	}
}
