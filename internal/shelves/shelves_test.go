package shelves

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/discover"
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

	connection := storetest.Open(t)

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
	fileless bool
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
	if !movie.fileless {
		if _, err := f.catalogue.SaveSource(ctx, items.MediaSource{
			SourceID: f.sourceID,
			ItemID:   record.ID,
			Path:     "/" + f.prefix + "/" + movie.name + ".mkv",
			Name:     movie.name,
		}); err != nil {
			t.Fatalf("failed to give %q a file: %v", movie.name, err)
		}
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

	tests := []serviceShelvesCase{
		{slug: "new-releases", now: october, wantName: "New Releases", want: []string{"Dud", "Quick", "Gem", "Seen", "Older"}},
		{slug: "hidden-gems", now: october, wantName: "Hidden Gems", want: []string{"Future", "Gem"}},
		{slug: "short-on-time", now: october, wantName: "Short on Time", want: []string{"Future", "Quick", "Dud"}},
		{slug: "spooky-season", now: october, wantName: "Spooky Season", want: []string{"Older", "Dud"}},
		{slug: "spooky-season", now: october.AddDate(0, 1, 0), absent: true},
		{slug: "because-you-watched", now: october, wantName: "Because You Watched Seen", want: []string{"Gem"}},
	}

	for _, test := range tests {
		t.Run(test.slug+" "+test.now.Month().String(), func(t *testing.T) {
			service := newService(fixture.catalogue, available(true), stored(test.slug), func() time.Time { return test.now })

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

type serviceShelvesCase struct {
	slug     string
	now      time.Time
	wantName string
	want     []string
	absent   bool
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
		service := newService(fixture.catalogue, available(true), stored("genre-spotlight"), func() time.Time { return now })

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

type settings struct {
	value json.RawMessage
}

func stored(slugs ...string) *settings {
	value, _ := json.Marshal(slugs)

	return &settings{value: value}
}

func (s *settings) Configuration(context.Context, string) (json.RawMessage, error) {
	return s.value, nil
}

func (s *settings) SetConfiguration(_ context.Context, _ string, value json.RawMessage) error {
	s.value = value

	return nil
}

func TestService_Choices(t *testing.T) {
	ctx := context.Background()
	service := newService(nil, available(true), &settings{}, time.Now)

	on := func() []string {
		choices, err := service.Choices(ctx)
		if err != nil {
			t.Fatalf("failed to list the choices: %v", err)
		}
		if len(choices) != len(definitions) {
			t.Fatalf("choices = %d, want every one of the %d shelves", len(choices), len(definitions))
		}

		enabled := []string{}
		for _, choice := range choices {
			if choice.Name == "" {
				t.Errorf("%s has no name", choice.Slug)
			}
			if choice.Enabled {
				enabled = append(enabled, choice.Slug)
			}
		}

		return enabled
	}

	if got := on(); !slices.Equal(got, Defaults) {
		t.Errorf("unset = %v, want the defaults %v", got, Defaults)
	}

	if err := service.Enable(ctx, []string{"trending-shows", "recently-aired", "trending-shows"}); err != nil {
		t.Fatalf("failed to enable the shelves: %v", err)
	}
	if got, want := on(), []string{"recently-aired", "trending-shows"}; !slices.Equal(got, want) {
		t.Errorf("enabled = %v, want %v", got, want)
	}

	if err := service.Enable(ctx, []string{"nonsense"}); !errors.Is(err, ErrUnknown) {
		t.Errorf("an unknown shelf answered %v, want ErrUnknown", err)
	}
	if got, want := on(), []string{"recently-aired", "trending-shows"}; !slices.Equal(got, want) {
		t.Errorf("a refused save left %v, want %v", got, want)
	}

	if err := service.Enable(ctx, nil); err != nil {
		t.Fatalf("failed to turn every shelf off: %v", err)
	}
	if got := on(); len(got) != 0 {
		t.Errorf("none = %v, want no shelves", got)
	}
}

type available bool

func (a available) Enabled() bool { return bool(a) }

func (a available) Trending(context.Context, items.Kind) ([]discover.Title, error) {
	return nil, nil
}

type trendingStub struct {
	keys   []string
	failed bool
}

func (t trendingStub) Enabled() bool { return true }

func (t trendingStub) Trending(_ context.Context, kind items.Kind) ([]discover.Title, error) {
	if t.failed {
		return nil, errors.New("the provider is unreachable")
	}

	titles := make([]discover.Title, 0, len(t.keys))
	for _, key := range t.keys {
		titles = append(titles, discover.Title{Kind: kind, Key: key})
	}

	return titles, nil
}

func TestService_TrendingShelves(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	key := func(name string) string { return fixture.prefix + ":" + name }

	fixture.movie(t, film{name: "Second", rating: 7, minutes: 90, genre: "Drama"})
	fixture.movie(t, film{name: "First", rating: 7, minutes: 90, genre: "Drama"})
	fixture.movie(t, film{name: "Orphan", rating: 7, minutes: 90, genre: "Drama", fileless: true})

	everything := trendingStub{keys: []string{key("Missing"), key("First"), key("Orphan"), key("Second")}}

	tests := []serviceTrendingShelvesCase{
		{name: "only what the library holds with a file, in the provider's order", discovery: everything, viewer: items.Everyone, want: []string{"First", "Second"}},
		{name: "what the viewer cannot see is left out", discovery: everything, viewer: items.Viewer{Libraries: []uuid.UUID{uuid.New()}}, absent: true},
		{name: "nothing held leaves the shelf out", discovery: trendingStub{keys: []string{key("Missing"), key("Orphan")}}, viewer: items.Everyone, absent: true},
		{name: "a provider that cannot answer leaves the shelf out", discovery: trendingStub{failed: true}, viewer: items.Everyone, absent: true},
		{name: "no provider leaves the shelf out", discovery: available(false), viewer: items.Everyone, absent: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newService(fixture.catalogue, test.discovery, stored("trending-movies"), time.Now)

			listed, err := service.Shelves(ctx, test.viewer, fixture.userID)
			if err != nil {
				t.Fatalf("failed to list the shelves: %v", err)
			}
			if test.absent {
				if len(listed) != 0 {
					t.Fatalf("shelves = %v, want none", listed)
				}

				return
			}
			if len(listed) != 1 || listed[0].Name != "Trending Movies" {
				t.Fatalf("shelves = %v, want Trending Movies", listed)
			}

			latest, err := service.Latest(ctx, listed[0], 20, false)
			if err != nil {
				t.Fatalf("failed to read the shelf: %v", err)
			}
			got := make([]string, 0, len(latest))
			for _, entry := range latest {
				got = append(got, entry.Item.Name)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("trending = %v, want %v", got, test.want)
			}

			card, found, err := service.Card(ctx, listed[0].ID, test.viewer, uuid.Nil, 5)
			if err != nil || !found {
				t.Fatalf("card = %v, %v, want the trending card", found, err)
			}
			if card.Name != "Trending Movies" || card.Personal || card.Fresh != 0 {
				t.Errorf("card = %q personal %v fresh %d, want the shelf's name, shared, with no count", card.Name, card.Personal, card.Fresh)
			}
		})
	}
}

type serviceTrendingShelvesCase struct {
	name      string
	discovery Discovery
	viewer    items.Viewer
	want      []string
	absent    bool
}

func TestService_Titles(t *testing.T) {
	service := newService(nil, available(true), stored("recently-aired", "genre-spotlight", "because-you-watched", "hidden-gems"), time.Now)

	want := []Title{
		{ID: ID("recently-aired"), Name: "Recently Aired"},
		{ID: ID("hidden-gems"), Name: "Hidden Gems"},
	}
	got, err := service.Titles(context.Background())
	if err != nil {
		t.Fatalf("failed to list the titles: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}

func TestService_Card(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	november := time.Date(2026, time.November, 14, 0, 0, 0, 0, time.UTC)
	year := func(value int) time.Time { return time.Date(value, time.June, 1, 0, 0, 0, 0, time.UTC) }

	older := fixture.movie(t, film{name: "Older", rating: 8, minutes: 90, genre: "Drama", released: year(2010)})
	newer := fixture.movie(t, film{name: "Newer", rating: 8, minutes: 90, genre: "Drama", released: year(2024)})
	fixture.movie(t, film{name: "Bare", rating: 8, minutes: 90, genre: "Drama", released: year(2020)})
	fixture.movie(t, film{name: "This Week", rating: 8, minutes: 90, genre: "Drama", released: november.AddDate(0, 0, -3)})
	fixture.movie(t, film{name: "Next Week", rating: 8, minutes: 90, genre: "Drama", released: november.AddDate(0, 0, 3)})
	for tag, itemID := range map[string]uuid.UUID{"older": older, "newer": newer} {
		if err := fixture.catalogue.SaveImage(ctx, itemID, items.Image{Kind: items.ImageKindPrimary, URL: "https://cdn.invalid/" + tag, Tag: tag}); err != nil {
			t.Fatalf("failed to save %q: %v", tag, err)
		}
	}

	viewer := items.Viewer{Libraries: []uuid.UUID{fixture.libraryID}}

	for _, test := range serviceCardCases {
		t.Run(test.name, func(t *testing.T) {
			service := newService(fixture.catalogue, available(true), stored(test.enabled...), func() time.Time { return november })

			card, found, err := service.Card(ctx, ID(test.slug), viewer, test.user, 10)
			if err != nil {
				t.Fatalf("failed to read the card: %v", err)
			}
			if found != test.found {
				t.Fatalf("found = %v, want %v", found, test.found)
			}
			if card.Name != test.title || card.Fresh != test.fresh || card.Personal != test.personal {
				t.Errorf("card = %q with %d new, personal %v, want %q with %d, personal %v", card.Name, card.Fresh, card.Personal, test.title, test.fresh, test.personal)
			}

			got := make([]string, 0, len(card.Posters))
			for _, poster := range card.Posters {
				got = append(got, poster.Tag)
			}
			if test.personal {
				slices.Sort(got)
			}
			if !slices.Equal(got, test.want) && (len(got) != 0 || len(test.want) != 0) {
				t.Errorf("posters = %v, want %v", got, test.want)
			}
		})
	}
}

type serviceCardCase struct {
	name     string
	enabled  []string
	slug     string
	user     uuid.UUID
	want     []string
	title    string
	fresh    int
	found    bool
	personal bool
}

var serviceCardCases = []serviceCardCase{
	{name: "a shelf everyone sees alike answers its name, what aired this week and its titles' posters newest first", enabled: []string{"new-releases"}, slug: "new-releases", want: []string{"newer", "older"}, title: "New Releases", fresh: 1, found: true},
	{name: "a shelf everyone sees alike is the same card whoever asks", enabled: []string{"new-releases"}, slug: "new-releases", user: uuid.New(), want: []string{"newer", "older"}, title: "New Releases", fresh: 1, found: true},
	{name: "a shelf that differs per user has none without a user", enabled: []string{"hidden-gems"}, slug: "hidden-gems"},
	{name: "a shelf that differs per user is that user's card", enabled: []string{"hidden-gems"}, slug: "hidden-gems", user: uuid.New(), want: []string{"newer", "older"}, title: "Hidden Gems", fresh: 1, found: true, personal: true},
	{name: "a shelf that is off has none", enabled: []string{"recently-aired"}, slug: "new-releases"},
}
