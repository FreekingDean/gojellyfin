package metadata

import (
	"context"
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

func (f *fixture) identify(t *testing.T) {
	t.Helper()

	if err := jobs.RunStep(t, f.service.IdentifyItems); err != nil {
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

func TestIdentifyWritesAMovie(t *testing.T) {
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
}

func TestIdentifyWalksASeriesToItsEpisode(t *testing.T) {
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
	fixed.identify(t)

	identified := fixed.reload(t, series.ID)
	if identified.ProviderIds["Stub"] != "1396" {
		t.Errorf("series provider id = %q, want 1396", identified.ProviderIds["Stub"])
	}
	if identified.Status != "Ended" {
		t.Errorf("series Status = %q, want Ended", identified.Status)
	}

	aired := fixed.reload(t, episode.ID)
	if aired.Name != "Pilot" {
		t.Errorf("episode Name = %q, want Pilot", aired.Name)
	}
	if aired.ProviderIds["StubExternal"] != "tt0959621" {
		t.Errorf("episode external id = %q, want tt0959621", aired.ProviderIds["StubExternal"])
	}
}

func TestIdentifyLeavesALockedItemAlone(t *testing.T) {
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
}

func TestIdentifyKeepsALockedField(t *testing.T) {
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
}

func TestIdentifyDoesNothingWithoutAProvider(t *testing.T) {
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
}

func TestIdentifyLeavesAnUnmatchedItemForTheNextRun(t *testing.T) {
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
}
