package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

func TestScanner_scanMovies(t *testing.T) {
	t.Run("fails on a missing root before it reaches a service", func(t *testing.T) {
		scanner := New(nil, nil, nil, nil)
		library := &libraries.Library{ID: uuid.New()}
		found := &seen{}

		if err := scanner.scanMovies(context.Background(), library, filepath.Join(t.TempDir(), "unmounted"), found); err == nil {
			t.Fatal("a missing root scanned clean, which the caller reads as an empty library")
		}
	})

	t.Run("skips an unreadable directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a directory whatever its mode")
		}

		root := t.TempDir()
		locked := filepath.Join(root, "locked")
		if err := os.Mkdir(locked, 0o000); err != nil {
			t.Fatalf("failed to create the directory: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

		scanner := New(nil, nil, nil, nil)
		library := &libraries.Library{ID: uuid.New()}
		found := &seen{}

		if err := scanner.scanMovies(context.Background(), library, root, found); err != nil {
			t.Fatalf("an unreadable directory failed the whole library: %v", err)
		}
		if found.complete() {
			t.Error("the walk reported complete, so the caller would sweep files it never reached")
		}
	})
}

func TestWalk_complete(t *testing.T) {
	found := &seen{}
	if !found.complete() {
		t.Error("a walk that skipped nothing reported incomplete")
	}

	found.file("/library/readable/movie.mkv")
	if !found.complete() {
		t.Error("finding a file made the walk incomplete")
	}

	found.skip("/library/unreadable", os.ErrPermission)
	if found.complete() {
		t.Error("a skipped directory left the walk reporting complete")
	}
}
