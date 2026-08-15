package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
)

type fixture struct {
	items     *items.Service
	provider  *Provider
	libraryID uuid.UUID

	mutex sync.Mutex
	asked []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	return newFixtureWithKey(t, "test-key")
}

func newFixtureWithKey(t *testing.T, apiKey string) *fixture {
	t.Helper()

	connection, err := store.NewStore()
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

	fixed := &fixture{items: items.New(client), libraryID: library.ID}

	server := httptest.NewServer(http.HandlerFunc(fixed.answer))
	t.Cleanup(server.Close)

	fixed.provider = New(newClient(server.URL, apiKey), fixed.items)

	return fixed
}

func (f *fixture) answer(writer http.ResponseWriter, request *http.Request) {
	f.mutex.Lock()
	f.asked = append(f.asked, request.URL.Path+"?"+request.URL.Query().Get("query"))
	f.mutex.Unlock()

	body, found := details[request.URL.Path]
	if strings.HasPrefix(request.URL.Path, "/3/search/") {
		body, found = searches[request.URL.Query().Get("query")]
	}
	if !found {
		http.Error(writer, `{"success":false}`, http.StatusNotFound)

		return
	}

	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(body))
}

func (f *fixture) requests() []string {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return append([]string(nil), f.asked...)
}

func (f *fixture) add(t *testing.T, scanned items.Scanned) *items.Item {
	t.Helper()

	scanned.LibraryID = f.libraryID
	if scanned.SortName == "" {
		scanned.SortName = scanned.Name
	}
	if scanned.Path == "" {
		scanned.Path = "/" + f.libraryID.String() + "/" + scanned.Name
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

	if err := jobs.RunStep(t, f.provider.IdentifyItems); err != nil {
		t.Fatalf("identification failed: %v", err)
	}
}

func number(value int32) *int32 {
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
		ProductionYear: number(1999),
	})

	fixed.identify(t)

	identified := fixed.reload(t, movie.ID)
	if identified.ProviderIds[providerTmdb] != "603" {
		t.Errorf("Tmdb id = %q, want 603", identified.ProviderIds[providerTmdb])
	}
	if identified.ProviderIds[providerImdb] != "tt0133093" {
		t.Errorf("Imdb id = %q, want tt0133093", identified.ProviderIds[providerImdb])
	}
	if identified.OfficialRating != "R" {
		t.Errorf("OfficialRating = %q, want R", identified.OfficialRating)
	}
	if identified.CommunityRating == nil || *identified.CommunityRating != 8.2 {
		t.Errorf("CommunityRating = %v, want 8.2", identified.CommunityRating)
	}
	if !strings.HasPrefix(identified.Overview, "Set in the 22nd century") {
		t.Errorf("Overview = %q, want the fetched one", identified.Overview)
	}
	if identified.PremiereDate == nil || identified.PremiereDate.Year() != 1999 {
		t.Errorf("PremiereDate = %v, want 1999-03-30", identified.PremiereDate)
	}
	if len(identified.Taglines) != 1 || identified.Taglines[0] != "Welcome to the Real World." {
		t.Errorf("Taglines = %v, want the fetched one", identified.Taglines)
	}
}

func TestIdentifyWalksASeriesToItsEpisode(t *testing.T) {
	fixed := newFixture(t)
	series := fixed.add(t, items.Scanned{
		Kind:           itemmodal.KindSeries,
		Name:           "Breaking Bad",
		ProductionYear: number(2008),
	})
	season := fixed.add(t, items.Scanned{
		Kind:        itemmodal.KindSeason,
		ParentID:    &series.ID,
		Name:        "Season 1",
		IndexNumber: number(1),
	})
	episode := fixed.add(t, items.Scanned{
		Kind:              itemmodal.KindEpisode,
		ParentID:          &season.ID,
		Name:              "s01e01",
		IndexNumber:       number(1),
		ParentIndexNumber: number(1),
	})

	// The episode is looked up under its series' id, so it takes the run after
	// the one that identified the series.
	fixed.identify(t)
	fixed.identify(t)

	identified := fixed.reload(t, series.ID)
	if identified.ProviderIds[providerTmdb] != "1396" {
		t.Errorf("series Tmdb id = %q, want 1396", identified.ProviderIds[providerTmdb])
	}
	if identified.OfficialRating != "TV-MA" {
		t.Errorf("series OfficialRating = %q, want TV-MA", identified.OfficialRating)
	}
	if identified.EndDate == nil || identified.EndDate.Year() != 2013 {
		t.Errorf("series EndDate = %v, want 2013-09-29", identified.EndDate)
	}

	aired := fixed.reload(t, episode.ID)
	if aired.Name != "Pilot" {
		t.Errorf("episode Name = %q, want Pilot", aired.Name)
	}
	if aired.ProviderIds[providerImdb] != "tt0959621" {
		t.Errorf("episode Imdb id = %q, want tt0959621", aired.ProviderIds[providerImdb])
	}
}

func TestIdentifyLeavesALockedItemAlone(t *testing.T) {
	fixed := newFixture(t)
	locked := fixed.lock(t, fixed.add(t, items.Scanned{
		Kind:           itemmodal.KindMovie,
		Name:           "The Matrix",
		ProductionYear: number(1999),
	}), items.Metadata{LockData: truth(true)})

	// A second, unlocked item proves the run did something, so the locked one
	// being untouched is not just an empty batch.
	witness := fixed.add(t, items.Scanned{
		Kind:           itemmodal.KindMovie,
		Name:           "The Matrix",
		Path:           "/" + fixed.libraryID.String() + "/witness",
		ProductionYear: number(1999),
	})

	fixed.identify(t)

	untouched := fixed.reload(t, locked.ID)
	if untouched.ProviderIds != nil {
		t.Errorf("ProviderIds = %v, want a locked item left alone", untouched.ProviderIds)
	}
	if untouched.Overview != "" {
		t.Errorf("Overview = %q, want a locked item left alone", untouched.Overview)
	}
	if fixed.reload(t, witness.ID).ProviderIds[providerTmdb] != "603" {
		t.Fatal("the run identified nothing, so the lock proves nothing")
	}
}

func TestIdentifyKeepsALockedField(t *testing.T) {
	fixed := newFixture(t)
	movie := fixed.lock(t, fixed.add(t, items.Scanned{
		Kind:           itemmodal.KindMovie,
		Name:           "The Matrix",
		ProductionYear: number(1999),
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
	if identified.ProviderIds[providerTmdb] != "603" {
		t.Errorf("Tmdb id = %q, want identity written through a field lock", identified.ProviderIds[providerTmdb])
	}
}

func TestIdentifyDoesNothingWithoutAKey(t *testing.T) {
	fixed := newFixtureWithKey(t, "")
	movie := fixed.add(t, items.Scanned{
		Kind:           itemmodal.KindMovie,
		Name:           "The Matrix",
		ProductionYear: number(1999),
	})

	fixed.identify(t)

	if asked := fixed.requests(); len(asked) != 0 {
		t.Errorf("requests = %v, want none without a key", asked)
	}
	if identified := fixed.reload(t, movie.ID); identified.ProviderIds != nil {
		t.Errorf("ProviderIds = %v, want nothing written without a key", identified.ProviderIds)
	}
}
