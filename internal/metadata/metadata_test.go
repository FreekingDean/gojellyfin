package metadata

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/consts"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
)

const unreachable = "A Film The Provider Cannot Reach"

var errUnreachable = errors.New("the provider is unreachable")

type stubProvider struct {
	enabled bool

	mutex sync.Mutex
	asked []string
}

func (s *stubProvider) Enabled() bool { return s.enabled }

func (s *stubProvider) record(what string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.asked = append(s.asked, what)
}

func (s *stubProvider) requests() []string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return append([]string(nil), s.asked...)
}

func (s *stubProvider) Movie(_ context.Context, name string, _ *int32) (items.Metadata, bool, error) {
	s.record("movie:" + name)
	if name == unreachable {
		return items.Metadata{}, false, errUnreachable
	}
	if name != "The Matrix" {
		return items.Metadata{}, false, nil
	}

	premiere := time.Date(1999, time.March, 30, 0, 0, 0, 0, time.UTC)

	return items.Metadata{
		Name:            text("The Matrix"),
		Overview:        text("Set in the 22nd century."),
		OfficialRating:  rated("R"),
		CommunityRating: number(8.2),
		PremiereDate:    &premiere,
		Taglines:        &[]string{"Welcome to the Real World."},
		ProviderIds:     &map[string]string{"Stub": "603", "StubExternal": "tt0133093"},
	}, true, nil
}

func (s *stubProvider) Series(_ context.Context, name string, _ *int32) (items.Metadata, bool, error) {
	s.record("series:" + name)
	if name != "Breaking Bad" {
		return items.Metadata{}, false, nil
	}

	return items.Metadata{
		Name:        text("Breaking Bad"),
		Status:      text("Ended"),
		ProviderIds: &map[string]string{"Stub": "1396"},
	}, true, nil
}

func (s *stubProvider) Season(_ context.Context, series map[string]string, season int32) (items.Metadata, bool, error) {
	s.record("season:" + series["Stub"])
	if series["Stub"] != "1396" || season > 1 {
		return items.Metadata{}, false, nil
	}
	if season == 0 {
		return items.Metadata{
			Name:        text("Specials"),
			ProviderIds: &map[string]string{"Stub": "3577"},
		}, true, nil
	}

	aired := time.Date(2008, time.January, 20, 0, 0, 0, 0, time.UTC)

	return items.Metadata{
		Name:           text("Season 1"),
		Overview:       text("Walter White turns to a life of crime."),
		PremiereDate:   &aired,
		ProductionYear: index(2008),
		ProviderIds:    &map[string]string{"Stub": "3572"},
	}, true, nil
}

func (s *stubProvider) Episode(_ context.Context, series map[string]string, season, episode int32) (items.Metadata, bool, error) {
	s.record("episode:" + series["Stub"])
	if series["Stub"] != "1396" || season != 1 || episode != 1 {
		return items.Metadata{}, false, nil
	}

	return items.Metadata{
		Name:        text("Pilot"),
		ProviderIds: &map[string]string{"Stub": "62085", "StubExternal": "tt0959621"},
	}, true, nil
}

type fixture struct {
	items     *items.Service
	service   *Service
	provider  *stubProvider
	libraryID uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	return newFixtureEnabled(t, true)
}

func newFixtureEnabled(t *testing.T, enabled bool) *fixture {
	t.Helper()

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
	catalogue := libraries.New(client)
	library, err := catalogue.CreateLibrary(
		context.Background(),
		t.Name()+"-"+uuid.NewString(),
		librarymodal.CollectionTypeMovies,
		[]string{"/" + uuid.NewString()},
	)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if err := catalogue.DeleteLibrary(ctx, library.ID); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	provider := &stubProvider{enabled: enabled}
	service := items.New(client)

	return &fixture{
		items:     service,
		service:   New(provider, service),
		provider:  provider,
		libraryID: library.ID,
	}
}

func (f *fixture) add(t *testing.T, scanned items.Scanned) *items.Item {
	t.Helper()

	scanned.LibraryID = f.libraryID
	if scanned.SortName == "" {
		scanned.SortName = scanned.Name
	}
	if scanned.Key == "" {
		scanned.Key = "test:" + f.libraryID.String() + ":" + scanned.Name
	}

	added, err := f.items.SaveScanned(context.Background(), scanned)
	if err != nil {
		t.Fatalf("failed to add %q: %v", scanned.Name, err)
	}

	return added
}

func (f *fixture) lock(t *testing.T, added *items.Item, metadata items.Metadata) *items.Item {
	t.Helper()

	locked, err := f.items.UpdateMetadata(context.Background(), added.ID, metadata)
	if err != nil {
		t.Fatalf("failed to lock %q: %v", added.Name, err)
	}

	return locked
}

func (f *fixture) reload(t *testing.T, id uuid.UUID) *items.Item {
	t.Helper()

	reloaded, err := f.items.ItemByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to read the item back: %v", err)
	}

	return reloaded
}

func (f *fixture) identified(t *testing.T, added *items.Item, overview string) *items.Item {
	t.Helper()

	return f.lock(t, added, items.Metadata{
		Overview:    text(overview),
		ProviderIds: &map[string]string{"Stub": "603"},
	})
}

func (f *fixture) identify(t *testing.T) {
	t.Helper()

	f.run(t, jobs.Options{})
}

func (f *fixture) run(t *testing.T, options jobs.Options) {
	t.Helper()

	if options.Scope == uuid.Nil {
		options.Scope = f.libraryID
	}

	if err := jobs.RunStep(t, f.service.IdentifyItems, options); err != nil {
		t.Fatalf("identification failed: %v", err)
	}
}

func index(value int32) *int32 {
	return &value
}

func text(value string) *string {
	return &value
}

func rated(value string) *consts.Rating {
	official := consts.Rating(value)

	return &official
}

func number(value float64) *float64 {
	return &value
}

func truth(value bool) *bool {
	return &value
}

func TestService_IdentifyItems(t *testing.T) {
	t.Run("writes a movie", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		})

		fixed.identify(t)

		identified := fixed.reload(t, movie.ID)
		if identified.ProviderIds["Stub"] != "603" {
			t.Errorf("provider id = %q, want 603", identified.ProviderIds["Stub"])
		}
		if identified.OfficialRating != "R" {
			t.Errorf("OfficialRating = %q, want R", identified.OfficialRating)
		}
		if !strings.HasPrefix(identified.Overview, "Set in the 22nd century") {
			t.Errorf("Overview = %q, want the fetched one", identified.Overview)
		}
		if identified.PremiereDate == nil || identified.PremiereDate.Year() != 1999 {
			t.Errorf("PremiereDate = %v, want 1999", identified.PremiereDate)
		}
	})

	t.Run("walks a series to its episode", func(t *testing.T) {
		fixed := newFixture(t)
		series := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindSeries,
			Name:           "Breaking Bad",
			ProductionYear: index(2008),
		})
		season := fixed.add(t, items.Scanned{
			Kind:        itemmodal.KindSeason,
			ParentID:    &series.ID,
			Name:        "Season 1",
			IndexNumber: index(1),
		})
		episode := fixed.add(t, items.Scanned{
			Kind:              itemmodal.KindEpisode,
			ParentID:          &season.ID,
			Name:              "s01e01",
			IndexNumber:       index(1),
			ParentIndexNumber: index(1),
		})

		fixed.identify(t)

		identified := fixed.reload(t, series.ID)
		if identified.ProviderIds["Stub"] != "1396" {
			t.Errorf("series provider id = %q, want 1396", identified.ProviderIds["Stub"])
		}
		if identified.Status != "Ended" {
			t.Errorf("series Status = %q, want Ended", identified.Status)
		}

		numbered := fixed.reload(t, season.ID)
		if numbered.Name != "Season 1" {
			t.Errorf("season Name = %q, want Season 1", numbered.Name)
		}
		if !strings.HasPrefix(numbered.Overview, "Walter White turns") {
			t.Errorf("season Overview = %q, want the fetched one", numbered.Overview)
		}
		if numbered.PremiereDate == nil || numbered.PremiereDate.Year() != 2008 {
			t.Errorf("season PremiereDate = %v, want 2008", numbered.PremiereDate)
		}
		if numbered.ProviderIds["Stub"] != "3572" {
			t.Errorf("season provider id = %q, want 3572", numbered.ProviderIds["Stub"])
		}
		if numbered.SortName != "Season 1" {
			t.Errorf("season SortName = %q, want the scanned one kept", numbered.SortName)
		}

		aired := fixed.reload(t, episode.ID)
		if aired.Name != "Pilot" {
			t.Errorf("episode Name = %q, want Pilot", aired.Name)
		}
		if aired.ProviderIds["StubExternal"] != "tt0959621" {
			t.Errorf("episode external id = %q, want tt0959621", aired.ProviderIds["StubExternal"])
		}
	})

	t.Run("identifies a parent listed after its children", func(t *testing.T) {
		fixed := newFixture(t)
		series := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindSeries,
			Name:           "Breaking Bad",
			ProductionYear: index(2008),
		})
		season := fixed.add(t, items.Scanned{
			Kind:        itemmodal.KindSeason,
			ParentID:    &series.ID,
			Name:        "Season 1",
			IndexNumber: index(1),
		})
		episode := fixed.add(t, items.Scanned{
			Kind:              itemmodal.KindEpisode,
			ParentID:          &season.ID,
			Name:              "s01e01",
			IndexNumber:       index(1),
			ParentIndexNumber: index(1),
		})
		fixed.lock(t, series, items.Metadata{})

		fixed.identify(t)

		if identified := fixed.reload(t, season.ID); identified.ProviderIds["Stub"] != "3572" {
			t.Errorf("season provider id = %q, want the series identified first", identified.ProviderIds["Stub"])
		}
		if identified := fixed.reload(t, episode.ID); identified.Name != "Pilot" {
			t.Errorf("episode Name = %q, want the series identified first", identified.Name)
		}
	})

	t.Run("identifies specials as season zero", func(t *testing.T) {
		fixed := newFixture(t)
		series := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindSeries,
			Name:           "Breaking Bad",
			ProductionYear: index(2008),
		})
		specials := fixed.add(t, items.Scanned{
			Kind:        itemmodal.KindSeason,
			ParentID:    &series.ID,
			Name:        "Specials",
			IndexNumber: index(0),
		})

		fixed.identify(t)

		identified := fixed.reload(t, specials.ID)
		if identified.ProviderIds["Stub"] != "3577" {
			t.Errorf("specials provider id = %q, want 3577", identified.ProviderIds["Stub"])
		}
	})

	t.Run("keeps an unmatched season out of its episode", func(t *testing.T) {
		fixed := newFixture(t)
		series := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindSeries,
			Name:           "Breaking Bad",
			ProductionYear: index(2008),
		})
		season := fixed.add(t, items.Scanned{
			Kind:        itemmodal.KindSeason,
			ParentID:    &series.ID,
			Name:        "Season 9",
			IndexNumber: index(9),
		})
		episode := fixed.add(t, items.Scanned{
			Kind:              itemmodal.KindEpisode,
			ParentID:          &season.ID,
			Name:              "s01e01",
			IndexNumber:       index(1),
			ParentIndexNumber: index(1),
		})

		fixed.identify(t)

		if unmatched := fixed.reload(t, season.ID); unmatched.ProviderIds != nil {
			t.Errorf("season ProviderIds = %v, want a miss to write nothing", unmatched.ProviderIds)
		}
		if aired := fixed.reload(t, episode.ID); aired.Name != "Pilot" {
			t.Errorf("episode Name = %q, want an unmatched season not to reach it", aired.Name)
		}
	})

	t.Run("keeps a locked season field", func(t *testing.T) {
		fixed := newFixture(t)
		series := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindSeries,
			Name:           "Breaking Bad",
			ProductionYear: index(2008),
		})
		season := fixed.lock(t, fixed.add(t, items.Scanned{
			Kind:        itemmodal.KindSeason,
			ParentID:    &series.ID,
			Name:        "The One With The Chemistry",
			IndexNumber: index(1),
		}), items.Metadata{LockedFields: &[]string{"Name"}})

		fixed.identify(t)

		identified := fixed.reload(t, season.ID)
		if identified.Name != "The One With The Chemistry" {
			t.Errorf("season Name = %q, want the manual edit kept", identified.Name)
		}
		if !strings.HasPrefix(identified.Overview, "Walter White turns") {
			t.Errorf("season Overview = %q, want an unlocked field written", identified.Overview)
		}
		if identified.ProviderIds["Stub"] != "3572" {
			t.Errorf("season provider id = %q, want identity written through a field lock", identified.ProviderIds["Stub"])
		}
	})

	t.Run("leaves a locked item alone", func(t *testing.T) {
		fixed := newFixture(t)
		locked := fixed.lock(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		}), items.Metadata{LockData: truth(true)})

		witness := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			Key:            "test:" + fixed.libraryID.String() + ":witness",
			ProductionYear: index(1999),
		})

		fixed.identify(t)

		untouched := fixed.reload(t, locked.ID)
		if untouched.ProviderIds != nil {
			t.Errorf("ProviderIds = %v, want a locked item left alone", untouched.ProviderIds)
		}
		if untouched.Overview != "" {
			t.Errorf("Overview = %q, want a locked item left alone", untouched.Overview)
		}
		if fixed.reload(t, witness.ID).ProviderIds["Stub"] != "603" {
			t.Fatal("the run identified nothing, so the lock proves nothing")
		}
	})

	t.Run("keeps a locked field", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.lock(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		}), items.Metadata{
			Overview:     text("A summary somebody wrote by hand."),
			LockedFields: &[]string{"Overview"},
		})

		fixed.identify(t)

		identified := fixed.reload(t, movie.ID)
		if identified.Overview != "A summary somebody wrote by hand." {
			t.Errorf("Overview = %q, want the manual edit kept", identified.Overview)
		}
		if len(identified.Taglines) != 0 {
			t.Errorf("Taglines = %v, want the Overview lock to cover them", identified.Taglines)
		}
		if identified.OfficialRating != "R" {
			t.Errorf("OfficialRating = %q, want an unlocked field written", identified.OfficialRating)
		}
		if identified.ProviderIds["Stub"] != "603" {
			t.Errorf("provider id = %q, want identity written through a field lock", identified.ProviderIds["Stub"])
		}
	})

	t.Run("does nothing without a provider", func(t *testing.T) {
		fixed := newFixtureEnabled(t, false)
		movie := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		})

		fixed.identify(t)

		if asked := fixed.provider.requests(); len(asked) != 0 {
			t.Errorf("requests = %v, want none from a disabled provider", asked)
		}
		if identified := fixed.reload(t, movie.ID); identified.ProviderIds != nil {
			t.Errorf("ProviderIds = %v, want nothing written", identified.ProviderIds)
		}
	})

	t.Run("leaves an unmatched item for the next run", func(t *testing.T) {
		fixed := newFixture(t)
		unknown := fixed.add(t, items.Scanned{
			Kind: itemmodal.KindMovie,
			Name: "A Film Nobody Carries",
		})

		fixed.identify(t)

		if identified := fixed.reload(t, unknown.ID); identified.ProviderIds != nil {
			t.Errorf("ProviderIds = %v, want a miss to write nothing", identified.ProviderIds)
		}
		if asked := fixed.provider.requests(); len(asked) != 1 {
			t.Errorf("requests = %v, want the miss to have been asked once", asked)
		}
	})
}

func TestService_IdentifyItems_Force(t *testing.T) {
	t.Run("looks an identified item up again", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		}), "Whatever the last provider said.")

		fixed.run(t, jobs.Options{Force: true})

		refreshed := fixed.reload(t, movie.ID)
		if !strings.HasPrefix(refreshed.Overview, "Set in the 22nd century") {
			t.Errorf("Overview = %q, want the refetched one", refreshed.Overview)
		}
		if refreshed.OfficialRating != "R" {
			t.Errorf("OfficialRating = %q, want R", refreshed.OfficialRating)
		}
	})

	t.Run("leaves an identified item alone without it", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		}), "Whatever the last provider said.")

		fixed.identify(t)

		if left := fixed.reload(t, movie.ID); left.Overview != "Whatever the last provider said." {
			t.Errorf("Overview = %q, want an identified item left alone", left.Overview)
		}
		if requests := fixed.provider.requests(); len(requests) != 0 {
			t.Errorf("requests = %v, want nothing fetched", requests)
		}
	})

	t.Run("keeps a locked field", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.lock(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		}), items.Metadata{
			Overview:     text("A summary somebody wrote by hand."),
			LockedFields: &[]string{"Overview"},
			ProviderIds:  &map[string]string{"Stub": "603"},
		})

		fixed.run(t, jobs.Options{Force: true})

		refreshed := fixed.reload(t, movie.ID)
		if refreshed.Overview != "A summary somebody wrote by hand." {
			t.Errorf("Overview = %q, want the lock to survive a forced refresh", refreshed.Overview)
		}
		if refreshed.OfficialRating != "R" {
			t.Errorf("OfficialRating = %q, want an unlocked field refreshed", refreshed.OfficialRating)
		}
	})

	t.Run("carries on past an item the provider cannot reach", func(t *testing.T) {
		fixed := newFixture(t)
		unreachableItem := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind: itemmodal.KindMovie,
			Name: unreachable,
		}), "Whatever the last provider said.")
		reachable := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			Key:            "test:" + fixed.libraryID.String() + ":reachable",
			ProductionYear: index(1999),
		}), "Whatever the last provider said.")

		fixed.run(t, jobs.Options{Force: true})

		if failed := fixed.reload(t, unreachableItem.ID); failed.Overview != "Whatever the last provider said." {
			t.Errorf("Overview = %q, want a failed fetch to write nothing", failed.Overview)
		}
		if done := fixed.reload(t, reachable.ID); !strings.HasPrefix(done.Overview, "Set in the 22nd century") {
			t.Error("the run stopped at the unreachable item, want it to carry on")
		}
	})

	t.Run("refreshes past the old two hundred item cap", func(t *testing.T) {
		fixed := newFixture(t)

		ids := make([]uuid.UUID, 0, 205)
		for number := range 205 {
			added := fixed.identified(t, fixed.add(t, items.Scanned{
				Kind:           itemmodal.KindMovie,
				Name:           "The Matrix",
				Key:            "test:" + fixed.libraryID.String() + ":" + strconv.Itoa(number),
				ProductionYear: index(1999),
			}), "Whatever the last provider said.")
			ids = append(ids, added.ID)
		}

		fixed.run(t, jobs.Options{Force: true})

		for _, id := range ids {
			if refreshed := fixed.reload(t, id); !strings.HasPrefix(refreshed.Overview, "Set in the 22nd century") {
				t.Fatalf("Overview = %q, want one run to drain the whole library", refreshed.Overview)
			}
		}
	})
}

func TestService_IdentifyItems_Scope(t *testing.T) {
	t.Run("refreshes only the item it names", func(t *testing.T) {
		fixed := newFixture(t)
		asked := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		}), "Whatever the last provider said.")
		elsewhere := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			Key:            "test:" + fixed.libraryID.String() + ":elsewhere",
			ProductionYear: index(1999),
		}), "Whatever the last provider said.")

		fixed.run(t, jobs.Options{Force: true, Scope: asked.ID})

		if refreshed := fixed.reload(t, asked.ID); !strings.HasPrefix(refreshed.Overview, "Set in the 22nd century") {
			t.Errorf("Overview = %q, want the named item refreshed", refreshed.Overview)
		}
		if left := fixed.reload(t, elsewhere.ID); left.Overview != "Whatever the last provider said." {
			t.Errorf("Overview = %q, want an item outside the scope left alone", left.Overview)
		}
	})

	t.Run("follows a series down to its episodes", func(t *testing.T) {
		fixed := newFixture(t)
		series := fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindSeries,
			Name:           "Breaking Bad",
			ProductionYear: index(2008),
		})
		season := fixed.add(t, items.Scanned{
			Kind:        itemmodal.KindSeason,
			ParentID:    &series.ID,
			Name:        "Season 1",
			IndexNumber: index(1),
		})
		episode := fixed.add(t, items.Scanned{
			Kind:              itemmodal.KindEpisode,
			ParentID:          &season.ID,
			Name:              "s01e01",
			IndexNumber:       index(1),
			ParentIndexNumber: index(1),
		})
		elsewhere := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		}), "Whatever the last provider said.")

		scope := jobs.Options{Force: true, Scope: series.ID}
		fixed.run(t, scope)
		fixed.run(t, scope)

		if identified := fixed.reload(t, series.ID); identified.ProviderIds["Stub"] != "1396" {
			t.Errorf("series provider id = %q, want the scoped series identified", identified.ProviderIds["Stub"])
		}
		if aired := fixed.reload(t, episode.ID); aired.Name != "Pilot" {
			t.Errorf("episode Name = %q, want the episode under the scope identified", aired.Name)
		}
		if left := fixed.reload(t, elsewhere.ID); left.Overview != "Whatever the last provider said." {
			t.Errorf("Overview = %q, want an item outside the scope left alone", left.Overview)
		}
	})

	t.Run("refreshes everything in a library it names", func(t *testing.T) {
		fixed := newFixture(t)
		first := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			ProductionYear: index(1999),
		}), "Whatever the last provider said.")
		second := fixed.identified(t, fixed.add(t, items.Scanned{
			Kind:           itemmodal.KindMovie,
			Name:           "The Matrix",
			Key:            "test:" + fixed.libraryID.String() + ":second",
			ProductionYear: index(1999),
		}), "Whatever the last provider said.")

		fixed.run(t, jobs.Options{Force: true, Scope: fixed.libraryID})

		for _, id := range []uuid.UUID{first.ID, second.ID} {
			if refreshed := fixed.reload(t, id); !strings.HasPrefix(refreshed.Overview, "Set in the 22nd century") {
				t.Errorf("Overview = %q, want everything in the library refreshed", refreshed.Overview)
			}
		}
	})
}
