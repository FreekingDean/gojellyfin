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
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type movie struct {
	title string
	year  int32
	file  string
}

type fixture struct {
	scanner *Scanner
	items   *items.Service
	client  *store.Client
	record  *libraries.Library
	bound   int
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
		scanner: New(service, libraries.New(client), sources.New(client, config), filesystem.New(config), ffmpeg.New(), activity.New(client)),
		items:   service,
		client:  client,
		record:  record,
	}
}

func (f *fixture) radarr(t *testing.T, reported *[]movie) *fixture {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := make([]map[string]any, 0, len(*reported))
		for _, entry := range *reported {
			payload = append(payload, map[string]any{
				"title":   entry.title,
				"year":    entry.year,
				"path":    filepath.Dir(entry.file),
				"hasFile": true,
				"tags":    []int{},
				"movieFile": map[string]any{
					"path":      entry.file,
					"size":      1,
					"dateAdded": time.Now().UTC(),
				},
			})
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
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	f.bound++

	if err := f.client.LibrarySource.Create().
		SetSourceID(record.ID).
		SetLibraryID(f.record.ID).
		Exec(context.Background()); err != nil {
		t.Fatalf("failed to bind the source to the library: %v", err)
	}

	t.Cleanup(func() {
		if err := f.client.Source.DeleteOne(record).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	return f
}

func (f *fixture) scan(t *testing.T) []*items.Item {
	t.Helper()

	if err := f.scanner.scanLibrary(context.Background(), f.record); err != nil {
		t.Fatalf("failed to scan: %v", err)
	}

	records, err := f.items.ItemsInLibrary(context.Background(), f.record.ID)
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
		reported := []movie{{"The Matrix", 1999, filepath.Join(root, "Some Folder", "abc123.mkv")}}

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
		hd := []movie{{"Blade Runner", 1982, filepath.Join(folder, "1080p.mkv")}}
		uhd := []movie{{"Blade Runner", 1982, filepath.Join(folder, "4K.mkv")}}

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
			{"Alien", 1979, filepath.Join(root, "Alien", "alien.mkv")},
			{"Aliens", 1986, filepath.Join(root, "Aliens", "aliens.mkv")},
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
		reported := []movie{{"Heat", 1995, filepath.Join(root, "Heat", "heat.mkv")}}

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
