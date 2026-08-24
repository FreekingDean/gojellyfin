package items

import (
	"context"
	"slices"
	"testing"
	"time"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (f *fixture) scanned(t *testing.T, key, path string) *Item {
	t.Helper()

	ctx := context.Background()
	item, err := f.service.SaveScanned(ctx, Scanned{
		LibraryID:    f.libraryID,
		Kind:         itemmodal.KindMovie,
		Key:          key,
		Name:         "The Matrix",
		SortName:     "matrix",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save %q: %v", path, err)
	}

	source, err := f.service.SaveSource(ctx, ScannedSource{
		LibraryID: f.libraryID,
		ItemID:    item.ID,
		Path:      path,
		Name:      path,
	})
	if err != nil {
		t.Fatalf("failed to save the source of %q: %v", path, err)
	}
	if err := f.service.SaveProbe(ctx, item, source, Probe{Container: "mkv"}); err != nil {
		t.Fatalf("failed to probe %q: %v", path, err)
	}

	return item
}

// Two files that derive one key are two copies of one title, which is the whole
// point of the key: a 4K and a 1080p rip become one entry with two sources.
func TestOneKeyGroupsCopies(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	key := "movie:the-matrix:1999"
	first := fixture.scanned(t, key, "/media/4k/The Matrix.mkv")
	second := fixture.scanned(t, key, "/media/hd/The Matrix.mkv")

	if first.ID != second.ID {
		t.Fatalf("two copies became two items: %s and %s", first.ID, second.ID)
	}

	sources, err := fixture.service.MediaSources(ctx, first.ID)
	if err != nil {
		t.Fatalf("failed to query the sources: %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("sources = %d, want both copies on the one item", len(sources))
	}
}

func TestSweepDropsAMissingSourceAndKeepsTheItem(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	key := "movie:the-matrix:1999"
	item := fixture.scanned(t, key, "/media/4k/The Matrix.mkv")
	fixture.scanned(t, key, "/media/hd/The Matrix.mkv")

	err := fixture.service.DeleteSourcesNotInPaths(ctx, fixture.libraryID, []string{"/media/4k/The Matrix.mkv"})
	if err != nil {
		t.Fatalf("failed to sweep the sources: %v", err)
	}

	sources, err := fixture.service.MediaSources(ctx, item.ID)
	if err != nil {
		t.Fatalf("failed to query the sources: %v", err)
	}
	if len(sources) != 1 || sources[0].Path != "/media/4k/The Matrix.mkv" {
		t.Fatalf("sources = %d, want the surviving copy alone", len(sources))
	}
	if _, err := fixture.service.ItemByID(ctx, item.ID); err != nil {
		t.Errorf("losing one copy took the item and its watch state: %v", err)
	}
}

// What is outstanding is read back from the rows: a file the probe has never
// read and a file that changed since it did, and nothing else.
func TestSourcesNeedingProbeSelectsTheUnreadAndTheChanged(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	probed := fixture.scanned(t, "movie:the-matrix:1999", "/media/hd/The Matrix.mkv")
	unread, err := fixture.service.SaveSource(ctx, ScannedSource{
		LibraryID:    fixture.libraryID,
		ItemID:       probed.ID,
		Path:         "/media/4k/The Matrix.mkv",
		Name:         "The Matrix.mkv",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the unprobed source: %v", err)
	}

	outstanding, err := fixture.service.SourcesNeedingProbe(ctx, fixture.libraryID)
	if err != nil {
		t.Fatalf("failed to select the sources needing a probe: %v", err)
	}
	if len(outstanding) != 1 || outstanding[0] != unread.ID {
		t.Fatalf("outstanding = %v, want the file nothing has probed", outstanding)
	}

	changed, err := fixture.service.SaveSource(ctx, ScannedSource{
		LibraryID:    fixture.libraryID,
		ItemID:       probed.ID,
		Path:         "/media/hd/The Matrix.mkv",
		Name:         "The Matrix.mkv",
		DateModified: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to touch the probed source: %v", err)
	}

	outstanding, err = fixture.service.SourcesNeedingProbe(ctx, fixture.libraryID)
	if err != nil {
		t.Fatalf("failed to select the sources needing a probe: %v", err)
	}
	if len(outstanding) != 2 {
		t.Fatalf("outstanding = %v, want the changed file back in the selection", outstanding)
	}
	if !slices.Contains(outstanding, changed.ID) {
		t.Errorf("a file that changed since its probe was not selected")
	}
}
