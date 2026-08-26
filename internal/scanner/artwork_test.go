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

	"github.com/FreekingDean/gojellyfin/internal/env"
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

func newFixture(t *testing.T) *fixture {
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
	record, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(context.Background())
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

	return &fixture{scanner: New(service, libraries.New(client), filesystem.New()), items: service, record: record}
}

func (l *fixture) artwork(t *testing.T) []*items.Image {
	t.Helper()

	records, err := l.items.ItemsInLibrary(context.Background(), l.record.ID)
	if err != nil {
		t.Fatalf("failed to read the library back: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("items = %d, want the one movie scanned", len(records))
	}

	found, err := l.items.Images(context.Background(), records[0].ID)
	if err != nil {
		t.Fatalf("failed to read the images: %v", err)
	}

	return found
}

func writeImage(t *testing.T, path string) {
	t.Helper()

	picture := image.NewRGBA(image.Rect(0, 0, 4, 2))
	picture.Set(0, 0, color.White)

	buffer := &bytes.Buffer{}
	if err := png.Encode(buffer, picture); err != nil {
		t.Fatalf("failed to encode %q: %v", path, err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", path, err)
	}
}

func TestScanArtwork(t *testing.T) {
	t.Run("keeps a sidecar poster when the folder holds none", func(t *testing.T) {
		fixture := newFixture(t)
		root := t.TempDir()
		folder := filepath.Join(root, "The Matrix (1999)")
		if err := os.Mkdir(folder, 0o700); err != nil {
			t.Fatalf("failed to create the folder: %v", err)
		}
		if err := os.WriteFile(filepath.Join(folder, "The Matrix (1999).mkv"), []byte("x"), 0o600); err != nil {
			t.Fatalf("failed to write the movie: %v", err)
		}
		writeImage(t, filepath.Join(folder, "The Matrix (1999)-poster.jpg"))

		if err := fixture.scanner.scanMovies(context.Background(), fixture.record, root, &seen{}); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}

		found := fixture.artwork(t)
		if len(found) != 1 {
			t.Fatalf("images = %d, want the sidecar poster", len(found))
		}
		if found[0].Kind != imagemodal.KindPrimary || filepath.Base(found[0].Path) != "The Matrix (1999)-poster.jpg" {
			t.Errorf("image = %s %q, want the sidecar poster", found[0].Kind, found[0].Path)
		}
	})

	t.Run("prefers the sidecar poster over the folder's", func(t *testing.T) {
		fixture := newFixture(t)
		root := t.TempDir()
		folder := filepath.Join(root, "Heat (1995)")
		if err := os.Mkdir(folder, 0o700); err != nil {
			t.Fatalf("failed to create the folder: %v", err)
		}
		if err := os.WriteFile(filepath.Join(folder, "Heat (1995).mkv"), []byte("x"), 0o600); err != nil {
			t.Fatalf("failed to write the movie: %v", err)
		}
		writeImage(t, filepath.Join(folder, "Heat (1995)-poster.jpg"))
		writeImage(t, filepath.Join(folder, "poster.jpg"))
		writeImage(t, filepath.Join(folder, "fanart.jpg"))

		if err := fixture.scanner.scanMovies(context.Background(), fixture.record, root, &seen{}); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}

		found := fixture.artwork(t)
		byKind := map[items.ImageKind]string{}
		for _, record := range found {
			byKind[record.Kind] = filepath.Base(record.Path)
		}

		if byKind[imagemodal.KindPrimary] != "Heat (1995)-poster.jpg" {
			t.Errorf("primary = %q, want the sidecar to win", byKind[imagemodal.KindPrimary])
		}
		if byKind[imagemodal.KindBackdrop] != "fanart.jpg" {
			t.Errorf("backdrop = %q, want the folder's fanart", byKind[imagemodal.KindBackdrop])
		}
	})
}
