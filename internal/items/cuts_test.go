package items

import (
	"context"
	"testing"
	"time"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const (
	twoHours     = 120 * ticksPerMinute
	twoHoursPlus = twoHours + ticksPerMinute
	extendedCut  = twoHours + 40*ticksPerMinute
	palTransfer  = twoHours * 24 / 25
)

func (f *fixture) scanned(t *testing.T, key, path string, runtime int64) *Item {
	t.Helper()

	ctx := context.Background()
	item, err := f.service.SaveScanned(ctx, Scanned{
		LibraryID:    f.libraryID,
		Kind:         itemmodal.KindMovie,
		Key:          key,
		Name:         "The Matrix",
		SortName:     "matrix",
		DateModified: time.Now(),
		RunTimeTicks: runtime,
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
	if err := f.service.SaveProbe(ctx, item, source, Probe{Container: "mkv", RunTimeTicks: runtime}); err != nil {
		t.Fatalf("failed to probe %q: %v", path, err)
	}

	return item
}

// Two files that derive one key are two copies of one film, which is the whole
// point of the key: a 4K and a 1080p rip become one entry with two sources.
func TestOneKeyGroupsCopies(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	key := "movie:the-matrix:1999"
	first := fixture.scanned(t, key, "/media/4k/The Matrix.mkv", twoHours)
	second := fixture.scanned(t, key, "/media/hd/The Matrix.mkv", twoHoursPlus)

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

// An extended cut is a different film wearing the same name, so grouping it
// would hand the wrong runtime and the wrong resume position to whoever plays it.
func TestADivergentRuntimeStaysItsOwnItem(t *testing.T) {
	fixture := newFixture(t)

	key := "movie:the-matrix:1999"
	theatrical := fixture.scanned(t, key, "/media/The Matrix.mkv", twoHours)
	extended := fixture.scanned(t, key, "/media/The Matrix Extended.mkv", extendedCut)

	if theatrical.ID == extended.ID {
		t.Fatal("an extended cut became a version of the theatrical one")
	}
	if theatrical.Key != key {
		t.Errorf("the first cut seen = %q, want the bare key %q", theatrical.Key, key)
	}
	if extended.Key == key {
		t.Error("the extended cut took the bare key, which the theatrical one already holds")
	}

	// Scanning again has to land on the same two items rather than forking more.
	again := fixture.scanned(t, key, "/media/The Matrix Extended.mkv", extendedCut)
	if again.ID != extended.ID {
		t.Errorf("a rescan moved the extended cut from %s to %s", extended.ID, again.ID)
	}
}

// A PAL transfer runs 25/24 fast and is still the same cut, so a plain
// tolerance would split every European release off on its own.
func TestPalSpeedupIsTheSameCut(t *testing.T) {
	fixture := newFixture(t)

	key := "movie:the-matrix:1999"
	film := fixture.scanned(t, key, "/media/The Matrix.mkv", twoHours)
	pal := fixture.scanned(t, key, "/media/The Matrix PAL.mkv", palTransfer)

	if film.ID != pal.ID {
		t.Errorf("a PAL transfer became its own item: %s, want %s", pal.ID, film.ID)
	}
}

func TestSweepDropsAMissingSourceAndKeepsTheItem(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	key := "movie:the-matrix:1999"
	item := fixture.scanned(t, key, "/media/4k/The Matrix.mkv", twoHours)
	fixture.scanned(t, key, "/media/hd/The Matrix.mkv", twoHours)

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
