package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

type fixture struct {
	scanner   *Scanner
	items     *items.Service
	directory string
	item      *items.Item
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	client := connection.Client()
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.MediaStream.Delete().
			Where(streammodal.HasSourceWith(sourcemodal.HasItemWith(itemmodal.LibraryID(library.ID)))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the streams: %v", err)
		}
		if _, err := client.MediaSource.Delete().Where(sourcemodal.HasItemWith(itemmodal.LibraryID(library.ID))).Exec(ctx); err != nil {
			t.Errorf("failed to delete the sources: %v", err)
		}
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := items.New(client)
	directory := t.TempDir()
	path := filepath.Join(directory, "Blade Runner (1982).mkv")
	if err := os.WriteFile(path, []byte("not really a movie"), 0o600); err != nil {
		t.Fatalf("failed to write the movie: %v", err)
	}

	item, err := service.SaveScanned(ctx, items.Scanned{
		LibraryID: library.ID,
		Kind:      itemmodal.KindMovie,
		Name:      "Blade Runner",
		SortName:  "blade runner",
		Path:      path,
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	return &fixture{
		scanner:   New(service, libraries.New(client)),
		items:     service,
		directory: directory,
		item:      item,
	}
}

func (f *fixture) write(t *testing.T, name, body string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(f.directory, name), []byte(body), 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", name, err)
	}
}

func (f *fixture) subtitles(t *testing.T) []*items.MediaStream {
	t.Helper()

	source, err := f.items.MediaSource(context.Background(), f.item.ID)
	if err != nil {
		t.Fatalf("failed to query the media source: %v", err)
	}
	if source == nil {
		return nil
	}

	found := make([]*items.MediaStream, 0)
	for _, stream := range source.Edges.Streams {
		if stream.Kind == streammodal.KindSubtitle {
			found = append(found, stream)
		}
	}

	return found
}

func (f *fixture) hasSubtitles(t *testing.T) bool {
	t.Helper()

	item, err := f.items.ItemByID(context.Background(), f.item.ID)
	if err != nil {
		t.Fatalf("failed to reload the item: %v", err)
	}

	return item.HasSubtitles
}

func TestScanSubtitles(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	fixture.write(t, "Blade Runner (1982).en.srt", "1\n")
	fixture.write(t, "Blade Runner (1982).fr.forced.srt", "1\n")
	fixture.write(t, "Blade Runner (1982).jpg", "not a subtitle")
	fixture.write(t, "Some Other Movie.en.srt", "1\n")

	if err := fixture.scanner.scanSubtitles(ctx, fixture.item); err != nil {
		t.Fatalf("failed to scan the subtitles: %v", err)
	}

	found := fixture.subtitles(t)
	if len(found) != 2 {
		t.Fatalf("subtitles = %d, want 2", len(found))
	}
	if found[0].Language != "en" || found[1].Language != "fr" {
		t.Errorf("languages = %q, %q, want en, fr", found[0].Language, found[1].Language)
	}
	if !found[1].IsForced {
		t.Error("the fr track should be forced")
	}
	for _, stream := range found {
		if !stream.IsExternal {
			t.Errorf("stream %d should be external", stream.Index)
		}
		if stream.Path != filepath.Join(fixture.directory, filepath.Base(stream.Path)) {
			t.Errorf("path = %q, want it beside the movie", stream.Path)
		}
	}
	if !fixture.hasSubtitles(t) {
		t.Error("the item should have subtitles")
	}

	t.Run("rescanning keeps one row per file", func(t *testing.T) {
		if err := fixture.scanner.scanSubtitles(ctx, fixture.item); err != nil {
			t.Fatalf("failed to rescan the subtitles: %v", err)
		}

		if again := fixture.subtitles(t); len(again) != 2 {
			t.Fatalf("subtitles = %d, want 2", len(again))
		}
	})

	t.Run("drops a subtitle that left the disk", func(t *testing.T) {
		if err := os.Remove(filepath.Join(fixture.directory, "Blade Runner (1982).fr.forced.srt")); err != nil {
			t.Fatalf("failed to remove the subtitle: %v", err)
		}
		if err := fixture.scanner.scanSubtitles(ctx, fixture.item); err != nil {
			t.Fatalf("failed to rescan the subtitles: %v", err)
		}

		remaining := fixture.subtitles(t)
		if len(remaining) != 1 || remaining[0].Language != "en" {
			t.Fatalf("subtitles = %v, want the en track alone", remaining)
		}
	})

	t.Run("clears the flag when the last subtitle goes", func(t *testing.T) {
		if err := os.Remove(filepath.Join(fixture.directory, "Blade Runner (1982).en.srt")); err != nil {
			t.Fatalf("failed to remove the subtitle: %v", err)
		}
		if err := fixture.scanner.scanSubtitles(ctx, fixture.item); err != nil {
			t.Fatalf("failed to rescan the subtitles: %v", err)
		}

		if found := fixture.subtitles(t); len(found) != 0 {
			t.Errorf("subtitles = %d, want 0", len(found))
		}
		if fixture.hasSubtitles(t) {
			t.Error("the item should no longer have subtitles")
		}
	})
}
