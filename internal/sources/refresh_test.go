package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemsourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
	librarysourcemodel "github.com/FreekingDean/gojellyfin/internal/store/librarysource"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type movieFile struct {
	ID   int64  `json:"id,omitempty"`
	Path string `json:"path"`
}

type movie struct {
	ID      int64     `json:"id,omitempty"`
	TmdbID  int       `json:"tmdbId"`
	Title   string    `json:"title"`
	Year    int32     `json:"year"`
	Path    string    `json:"path"`
	Tags    []int     `json:"tags"`
	File    movieFile `json:"movieFile"`
	HasFile bool      `json:"hasFile"`
}

type fixture struct {
	service  *Service
	items    *items.Service
	client   *store.Client
	sourceID uuid.UUID
}

func newFixture(t *testing.T, reported *[]movie) *fixture {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}
	config.SourceAPIKeys = map[string]string{"SOURCE_API_KEY_TEST": "key"}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}
	client := connection.Client()

	radarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/tag":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 1, "label": "kids"},
				{"id": 2, "label": "grown"},
			})
		case "/api/v3/movie":
			_ = json.NewEncoder(w).Encode(*reported)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(radarr.Close)

	source, err := client.Source.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetURL(radarr.URL).
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindRadarr).
		SetRootPath("/media").
		SetLocalPath("/media").
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}

	records := items.New(client)

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.ItemSource.Delete().
			Where(itemsourcemodel.SourceID(source.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the files: %v", err)
		}
		if err := client.Source.DeleteOne(source).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{
		service:  New(client, config, records, activity.New(client)),
		items:    records,
		client:   client,
		sourceID: source.ID,
	}
}

func (f *fixture) library(t *testing.T, filter string) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	library, err := f.client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}
	if err := f.client.LibrarySource.Create().
		SetLibraryID(library.ID).
		SetSourceID(f.sourceID).
		SetTagFilter(filter).
		Exec(ctx); err != nil {
		t.Fatalf("failed to bind the source: %v", err)
	}
	t.Cleanup(func() {
		if err := f.client.Library.DeleteOne(library).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
	})

	return library.ID
}

func (f *fixture) refresh(t *testing.T, libraryID uuid.UUID) []jobs.Params {
	t.Helper()

	enqueued, err := jobs.RunJob(t, f.service.RefreshJob(),
		jobs.With(jobs.ParamLibrary, libraryID),
		jobs.With(jobs.ParamSource, f.sourceID),
	)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	return jobs.Enqueued(t, enqueued, jobs.RefreshItem)
}

func (f *fixture) write(t *testing.T, libraryID uuid.UUID, params []jobs.Params) {
	t.Helper()

	for _, held := range params {
		var scanned items.Scanned
		if err := json.Unmarshal([]byte(held[jobs.ParamItem]), &scanned); err != nil {
			t.Fatalf("failed to read the enqueued title: %v", err)
		}
		if err := f.items.RefreshItem(context.Background(), libraryID, f.sourceID, scanned); err != nil {
			t.Fatalf("failed to write %s: %v", scanned.Key, err)
		}
	}
}

func (f *fixture) files(t *testing.T) map[string]uuid.UUID {
	t.Helper()

	records, err := f.client.ItemSource.Query().
		Where(itemsourcemodel.SourceID(f.sourceID)).
		All(context.Background())
	if err != nil {
		t.Fatalf("failed to read the files back: %v", err)
	}

	held := make(map[string]uuid.UUID, len(records))
	for _, record := range records {
		held[record.Path] = record.ID
	}

	return held
}

func at(path string) movieFile {
	return movieFile{Path: path}
}

func TestRefreshLibrarySource_SharedSource(t *testing.T) {
	reported := []movie{
		{TmdbID: 8844, Title: "Jumanji", Year: 1995, HasFile: true,
			Path: "/media/Jumanji", File: at("/media/Jumanji/jumanji.mkv"), Tags: []int{1}},
		{TmdbID: 949, Title: "Heat", Year: 1995, HasFile: true,
			Path: "/media/Heat", File: at("/media/Heat/heat.mkv"), Tags: []int{2}},
	}

	fixture := newFixture(t, &reported)
	kids := fixture.library(t, "kids")
	grown := fixture.library(t, "grown")

	var before map[string]uuid.UUID
	var after map[string]uuid.UUID

	for _, run := range []struct {
		library uuid.UUID
		want    string
	}{{kids, items.MovieKey(8844)}, {grown, items.MovieKey(949)}} {
		enqueued := fixture.refresh(t, run.library)
		if len(enqueued) != 2 {
			t.Fatalf("enqueued = %d, want every title the source holds", len(enqueued))
		}
		fixture.write(t, run.library, enqueued)

		members, err := fixture.client.LibraryItem.Query().
			Where(librarymembership.LibraryID(run.library)).
			WithItem().
			AllModels(context.Background())
		if err != nil {
			t.Fatalf("failed to read the membership: %v", err)
		}
		if len(members) != 1 || members[0].Item.Key != run.want {
			t.Fatalf("membership = %d rows, want only the tagged title", len(members))
		}

		before, after = after, fixture.files(t)
	}

	if len(after) != 2 {
		t.Fatalf("files = %v, want both copies the source holds", after)
	}
	for path, id := range before {
		if after[path] != id {
			t.Errorf("%s was deleted and rewritten by the other library's refresh, "+
				"so it lost its probe and will be read again every run", path)
		}
	}
}

func (f *fixture) binding(t *testing.T, libraryID uuid.UUID) *store.LibrarySourceModel {
	t.Helper()

	record, err := f.client.LibrarySource.Query().
		Where(
			librarysourcemodel.LibraryID(libraryID),
			librarysourcemodel.SourceID(f.sourceID),
		).
		OnlyModel(context.Background())
	if err != nil {
		t.Fatalf("failed to read the binding back: %v", err)
	}

	return record
}

func TestRefreshLibrarySource_RecordsTheIDsTheInstanceAnswered(t *testing.T) {
	reported := []movie{
		{ID: 41, TmdbID: 603, Title: "The Matrix", Year: 1999, HasFile: true,
			Path: "/media/Matrix", File: movieFile{ID: 4100, Path: "/media/Matrix/matrix.mkv"}},
		{TmdbID: 949, Title: "Heat", Year: 1995, HasFile: true,
			Path: "/media/Heat", File: at("/media/Heat/heat.mkv")},
	}

	fixture := newFixture(t, &reported)
	library := fixture.library(t, "")
	fixture.write(t, library, fixture.refresh(t, library))

	records, err := fixture.client.ItemSource.Query().
		Where(itemsourcemodel.SourceID(fixture.sourceID)).
		AllModels(context.Background())
	if err != nil {
		t.Fatalf("failed to read the files back: %v", err)
	}

	byPath := map[string]*store.ItemSourceModel{}
	for _, record := range records {
		byPath[record.Path] = record
	}

	matrix := byPath["/media/Matrix/matrix.mkv"]
	if matrix == nil || matrix.ExternalID == nil || *matrix.ExternalID != 41 {
		t.Errorf("matrix external id = %v, want 41", matrix)
	}
	if matrix == nil || matrix.ExternalFileID == nil || *matrix.ExternalFileID != 4100 {
		t.Errorf("matrix external file id = %v, want 4100", matrix)
	}

	heat := byPath["/media/Heat/heat.mkv"]
	if heat == nil || heat.ExternalID != nil || heat.ExternalFileID != nil {
		t.Errorf("heat = %+v, want no external ids for an instance that answered none", heat)
	}
}

func TestRefreshLibrarySource_RecordsTheSync(t *testing.T) {
	reported := []movie{
		{TmdbID: 949, Title: "Heat", Year: 1995, HasFile: true,
			Path: "/media/Heat", File: at("/media/Heat/heat.mkv")},
	}

	fixture := newFixture(t, &reported)

	t.Run("a read that succeeds stamps the time and clears the error", func(t *testing.T) {
		library := fixture.library(t, "")
		if err := fixture.client.LibrarySource.Update().
			Where(librarysourcemodel.LibraryID(library)).
			SetLastError("earlier failure").
			Exec(context.Background()); err != nil {
			t.Fatalf("failed to seed an error: %v", err)
		}

		fixture.refresh(t, library)

		binding := fixture.binding(t, library)
		if binding.LastSyncedAt == nil {
			t.Error("last synced at = nil, want the time of the read")
		}
		if binding.LastError != "" {
			t.Errorf("last error = %q, want it cleared", binding.LastError)
		}
	})

	t.Run("a read that fails keeps the last success and says why", func(t *testing.T) {
		library := fixture.library(t, "missing")

		if _, err := jobs.RunJob(t, fixture.service.RefreshJob(),
			jobs.With(jobs.ParamLibrary, library),
			jobs.With(jobs.ParamSource, fixture.sourceID),
		); err == nil {
			t.Fatal("refresh succeeded, want the unknown tag to fail it")
		}

		binding := fixture.binding(t, library)
		if binding.LastSyncedAt != nil {
			t.Errorf("last synced at = %v, want nil: nothing has synced", binding.LastSyncedAt)
		}
		if binding.LastError == "" {
			t.Error("last error is empty, want the reason the read failed")
		}
	})
}

func TestRefreshLibrarySource_RecordsACancelledRead(t *testing.T) {
	reported := []movie{}
	fixture := newFixture(t, &reported)
	library := fixture.library(t, "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	binding := fixture.binding(t, library)
	if err := fixture.service.recordSync(ctx, binding.ID, context.Canceled); err != nil {
		t.Fatalf("failed to record a cancelled read: %v", err)
	}

	if got := fixture.binding(t, library).LastError; got != context.Canceled.Error() {
		t.Errorf("last error = %q, want %q", got, context.Canceled.Error())
	}
}
