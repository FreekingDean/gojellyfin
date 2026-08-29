package scanner

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
)

type fixture struct {
	scanner *Scanner
	items   *items.Service
	record  *libraries.Library
}

func newFixture(t *testing.T, root string) *fixture {
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

	return &fixture{scanner: New(service, libraries.New(client), filesystem.New(config), ffmpeg.New(), activity.New(client)), items: service, record: record}
}

func (f *fixture) scan(t *testing.T) map[items.ImageKind]string {
	t.Helper()

	if err := f.scanner.scanLibrary(context.Background(), f.record); err != nil {
		t.Fatalf("failed to scan: %v", err)
	}

	records, err := f.items.ItemsInLibrary(context.Background(), f.record.ID)
	if err != nil {
		t.Fatalf("failed to read the library back: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("items = %d, want the one movie scanned", len(records))
	}

	found, err := f.items.Images(context.Background(), records[0].ID)
	if err != nil {
		t.Fatalf("failed to read the images: %v", err)
	}

	artwork := map[items.ImageKind]string{}
	for _, record := range found {
		artwork[record.Kind] = filepath.Base(record.Path)
	}

	return artwork
}

func movieFolder(t *testing.T, root, name string, files ...string) {
	t.Helper()

	folder := filepath.Join(root, name)
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatalf("failed to create %q: %v", name, err)
	}

	for _, file := range files {
		path := filepath.Join(folder, file)
		if isImage(file) {
			picture := image.NewRGBA(image.Rect(0, 0, 4, 2))
			picture.Set(0, 0, color.White)

			buffer := &bytes.Buffer{}
			if err := png.Encode(buffer, picture); err != nil {
				t.Fatalf("failed to encode %q: %v", file, err)
			}
			if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
				t.Fatalf("failed to write %q: %v", file, err)
			}

			continue
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("failed to write %q: %v", file, err)
		}
	}
}

func TestScanArtwork(t *testing.T) {
	t.Run("keeps a sidecar poster when the folder holds none", func(t *testing.T) {
		root := t.TempDir()
		movieFolder(t, root, "The Matrix (1999)",
			"The Matrix (1999).mkv",
			"The Matrix (1999)-poster.jpg",
		)

		found := newFixture(t, root).scan(t)

		if found[imagemodal.KindPrimary] != "The Matrix (1999)-poster.jpg" {
			t.Errorf("primary = %q, want the sidecar poster", found[imagemodal.KindPrimary])
		}
	})

	t.Run("prefers the sidecar poster over the folder's", func(t *testing.T) {
		root := t.TempDir()
		movieFolder(t, root, "Heat (1995)",
			"Heat (1995).mkv",
			"Heat (1995)-poster.jpg",
			"poster.jpg",
			"fanart.jpg",
		)

		found := newFixture(t, root).scan(t)

		if found[imagemodal.KindPrimary] != "Heat (1995)-poster.jpg" {
			t.Errorf("primary = %q, want the sidecar to win", found[imagemodal.KindPrimary])
		}
		if found[imagemodal.KindBackdrop] != "fanart.jpg" {
			t.Errorf("backdrop = %q, want the folder's fanart", found[imagemodal.KindBackdrop])
		}
	})

	t.Run("keeps one file's poster when a second file of the same title has none", func(t *testing.T) {
		root := t.TempDir()
		movieFolder(t, root, "Blade Runner (1982)",
			"Blade Runner (1982) - 1080p.mkv",
			"Blade Runner (1982) - 1080p-poster.jpg",
			"Blade Runner (1982) - 4K.mkv",
		)

		found := newFixture(t, root).scan(t)

		if found[imagemodal.KindPrimary] != "Blade Runner (1982) - 1080p-poster.jpg" {
			t.Errorf("primary = %q, want the poster the first file carried", found[imagemodal.KindPrimary])
		}
	})

	t.Run("keeps the first file's poster when both files carry one", func(t *testing.T) {
		root := t.TempDir()
		movieFolder(t, root, "Sicario (2015)",
			"Sicario (2015) - 1080p.mkv",
			"Sicario (2015) - 1080p-poster.jpg",
			"Sicario (2015) - 4K.mkv",
			"Sicario (2015) - 4K-poster.jpg",
		)

		found := newFixture(t, root).scan(t)

		if found[imagemodal.KindPrimary] != "Sicario (2015) - 1080p-poster.jpg" {
			t.Errorf("primary = %q, want the first file walked to win", found[imagemodal.KindPrimary])
		}
	})

	t.Run("keeps artwork when the walk cannot read a directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a directory whatever its mode")
		}

		root := t.TempDir()
		movieFolder(t, root, "Arrival (2016)",
			"Arrival (2016).mkv",
			"poster.jpg",
		)

		fixture := newFixture(t, root)
		if found := fixture.scan(t); found[imagemodal.KindPrimary] != "poster.jpg" {
			t.Fatalf("primary = %q, want the folder poster", found[imagemodal.KindPrimary])
		}

		folder := filepath.Join(root, "Arrival (2016)")
		if err := os.Chmod(folder, 0o000); err != nil {
			t.Fatalf("failed to lock the folder: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(folder, 0o700) })

		if found := fixture.scan(t); found[imagemodal.KindPrimary] != "poster.jpg" {
			t.Errorf("primary = %q, want the poster a partial walk cannot see left alone", found[imagemodal.KindPrimary])
		}
	})

	t.Run("drops artwork the operator deleted from disk", func(t *testing.T) {
		root := t.TempDir()
		movieFolder(t, root, "Alien (1979)",
			"Alien (1979).mkv",
			"poster.jpg",
		)

		fixture := newFixture(t, root)
		if found := fixture.scan(t); found[imagemodal.KindPrimary] != "poster.jpg" {
			t.Fatalf("primary = %q, want the folder poster", found[imagemodal.KindPrimary])
		}

		if err := os.Remove(filepath.Join(root, "Alien (1979)", "poster.jpg")); err != nil {
			t.Fatalf("failed to delete the poster: %v", err)
		}

		if found := fixture.scan(t); len(found) != 0 {
			t.Errorf("images = %v, want the deleted poster gone", found)
		}
	})
}
