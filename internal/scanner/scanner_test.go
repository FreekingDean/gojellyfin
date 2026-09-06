package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemsourcemodal "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type movie struct {
	title string
	year  int32
	file  string
	tags  []int
}

type fixture struct {
	scanner  *Scanner
	items    *items.Service
	client   *store.Client
	record   *libraries.Library
	root     string
	bound    int
	sourceID uuid.UUID
	tags     map[string]int
}

func newFixture(t *testing.T, root string) *fixture {
	t.Helper()

	t.Setenv(env.SourceAPIKeyPrefix+"TEST", "key")

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
	record, err := client.Library.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetCollectionType(libraries.CollectionTypeMovies).
		SetLocations([]string{root}).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		if err := client.Library.DeleteOne(record).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := items.New(client)

	return &fixture{
		scanner: New(service, libraries.New(client), sources.New(client, config), ffmpeg.New(), activity.New(client)),
		items:   service,
		client:  client,
		record:  record,
		root:    root,
	}
}

func tagsOf(entry movie) []int {
	if entry.tags == nil {
		return []int{}
	}

	return entry.tags
}

func (f *fixture) radarr(t *testing.T, reported *[]movie) *fixture {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := make([]map[string]any, 0, len(*reported))
		for _, entry := range *reported {
			payload = append(payload, map[string]any{
				"title":   entry.title,
				"year":    entry.year,
				"path":    filepath.Dir(entry.file),
				"hasFile": true,
				"tags":    tagsOf(entry),
				"movieFile": map[string]any{
					"path":      entry.file,
					"size":      1,
					"dateAdded": time.Now().UTC(),
				},
			})
		}

		if r.URL.Path == "/"+arr.TagsPath {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 1, "label": "kids"}, {"id": 2, "label": "grown"},
			})

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(server.Close)

	record, err := f.client.Source.Create().
		SetName(fmt.Sprintf("%s-%d", f.record.Name, f.bound)).
		SetURL(server.URL).
		SetAPIKeyVariable(env.SourceAPIKeyPrefix + "TEST").
		SetKind(sourcemodal.KindRadarr).
		SetRootPath(f.root).
		SetLocalPath(f.root).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	f.bound++

	f.sourceID = record.ID
	f.bind(t, record.ID, "")

	t.Cleanup(func() {
		if err := f.client.Source.DeleteOne(record).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	return f
}

func (f *fixture) bind(t *testing.T, source uuid.UUID, tag string) {
	t.Helper()

	if err := f.client.LibrarySource.Create().
		SetSourceID(source).
		SetLibraryID(f.record.ID).
		SetTagFilter(tag).
		Exec(context.Background()); err != nil {
		t.Fatalf("failed to bind the source to the library: %v", err)
	}
}

func (f *fixture) tagged(t *testing.T, reported *[]movie, tag string) *fixture {
	t.Helper()

	f.tags = map[string]int{"kids": 1, "grown": 2}
	f.radarr(t, reported)
	if _, err := f.client.LibrarySource.Delete().Exec(context.Background()); err != nil {
		t.Fatalf("failed to clear the binding: %v", err)
	}
	f.bind(t, f.sourceID, tag)

	return f
}

func (f *fixture) scan(t *testing.T) []*items.Item {
	t.Helper()

	if _, err := f.scanner.scanLibrary(context.Background(), f.record); err != nil {
		t.Fatalf("failed to scan: %v", err)
	}

	records, _, err := f.items.QueryItems(context.Background(), items.ItemQuery{Viewer: items.Everyone, LibraryID: &f.record.ID})
	if err != nil {
		t.Fatalf("failed to read the library back: %v", err)
	}

	return records
}

func (f *fixture) paths(t *testing.T, itemID uuid.UUID) []string {
	t.Helper()

	sources, err := f.items.MediaSources(context.Background(), itemID)
	if err != nil {
		t.Fatalf("failed to read the media sources: %v", err)
	}

	found := make([]string, 0, len(sources))
	for _, source := range sources {
		found = append(found, source.Path)
	}

	return found
}

func TestScanLibrary(t *testing.T) {
	t.Run("takes the title and year from the source rather than the filename", func(t *testing.T) {
		root := t.TempDir()
		reported := []movie{{"The Matrix", 1999, filepath.Join(root, "Some Folder", "abc123.mkv"), nil}}

		records := newFixture(t, root).radarr(t, &reported).scan(t)

		if len(records) != 1 {
			t.Fatalf("items = %d, want the one movie the source reported", len(records))
		}
		if records[0].Name != "The Matrix" {
			t.Errorf("name = %q, want the source's title, not the filename", records[0].Name)
		}
		if records[0].Key != "movie:the-matrix:1999" {
			t.Errorf("key = %q, want it derived from the reported title and year", records[0].Key)
		}
		if year := records[0].ProductionYear; year == nil || *year != 1999 {
			t.Errorf("production year = %v, want 1999", year)
		}
	})

	t.Run("two sources reporting one title make one item with two files", func(t *testing.T) {
		root := t.TempDir()
		folder := filepath.Join(root, "Blade Runner (1982)")
		hd := []movie{{"Blade Runner", 1982, filepath.Join(folder, "1080p.mkv"), nil}}
		uhd := []movie{{"Blade Runner", 1982, filepath.Join(folder, "4K.mkv"), nil}}

		fixture := newFixture(t, root).radarr(t, &hd).radarr(t, &uhd)
		records := fixture.scan(t)

		if len(records) != 1 {
			t.Fatalf("items = %d, want the two copies collapsed onto one title", len(records))
		}
		if paths := fixture.paths(t, records[0].ID); len(paths) != 2 {
			t.Errorf("media sources = %v, want both files on the one item", paths)
		}
	})

	t.Run("sweeps a title the source stopped reporting", func(t *testing.T) {
		root := t.TempDir()
		reported := []movie{
			{"Alien", 1979, filepath.Join(root, "Alien", "alien.mkv"), nil},
			{"Aliens", 1986, filepath.Join(root, "Aliens", "aliens.mkv"), nil},
		}

		fixture := newFixture(t, root).radarr(t, &reported)
		if records := fixture.scan(t); len(records) != 2 {
			t.Fatalf("items = %d, want both titles", len(records))
		}

		reported = reported[:1]

		records := fixture.scan(t)
		if len(records) != 1 {
			t.Fatalf("items = %d, want the dropped title swept", len(records))
		}
		if records[0].Name != "Alien" {
			t.Errorf("name = %q, want the title the source still reports", records[0].Name)
		}
	})

	t.Run("a library no source is bound to is left alone", func(t *testing.T) {
		root := t.TempDir()
		reported := []movie{{"Heat", 1995, filepath.Join(root, "Heat", "heat.mkv"), nil}}

		fixture := newFixture(t, root).radarr(t, &reported)
		if records := fixture.scan(t); len(records) != 1 {
			t.Fatalf("items = %d, want the seeded title", len(records))
		}

		if _, err := fixture.client.LibrarySource.Delete().Exec(context.Background()); err != nil {
			t.Fatalf("failed to unbind the source: %v", err)
		}

		if records := fixture.scan(t); len(records) != 1 {
			t.Errorf("items = %d, want the unbound library left alone rather than emptied", len(records))
		}
	})
}

func TestScanLibrary_SharedSource(t *testing.T) {
	root := t.TempDir()
	kids := filepath.Join(root, "Jumanji", "jumanji.mkv")
	grown := filepath.Join(root, "Heat", "heat.mkv")

	reported := []movie{
		{"Jumanji", 1995, kids, []int{1}},
		{"Heat", 1995, grown, []int{2}},
	}

	first := newFixture(t, root)
	first.tagged(t, &reported, "kids")

	second := newFixture(t, root)
	second.bind(t, first.sourceID, "grown")

	if records := first.scan(t); len(records) != 1 || records[0].Name != "Jumanji" {
		t.Fatalf("first library = %v, want only the kids title", names(records))
	}
	if records := second.scan(t); len(records) != 1 || records[0].Name != "Heat" {
		t.Fatalf("second library = %v, want only the grown title", names(records))
	}

	files, err := first.client.ItemSource.Query().
		Where(itemsourcemodal.SourceID(first.sourceID)).
		All(context.Background())
	if err != nil {
		t.Fatalf("failed to read the files back: %v", err)
	}
	if len(files) != 2 {
		held := make([]string, 0, len(files))
		for _, file := range files {
			held = append(held, file.Path)
		}
		t.Errorf("files = %v, want both: one library's scan swept the shared source's other files", held)
	}
}

func names(records []*items.Item) []string {
	found := make([]string, 0, len(records))
	for _, record := range records {
		found = append(found, record.Name)
	}

	return found
}
