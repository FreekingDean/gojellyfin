package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

// Nothing is reached that needs a service: the walk fails on its first call.
func TestScanMoviesFailsOnAMissingRoot(t *testing.T) {
	scanner := New(nil, nil, nil)
	library := &libraries.Library{ID: uuid.New()}

	paths, err := scanner.scanMovies(context.Background(), library, filepath.Join(t.TempDir(), "unmounted"))
	if err == nil {
		t.Fatalf("a missing root scanned clean and returned %d paths, which the caller reads as an empty library", len(paths))
	}
}

func TestScanMoviesFailsOnAnUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory whatever its mode")
	}

	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatalf("failed to create the directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	scanner := New(nil, nil, nil)
	library := &libraries.Library{ID: uuid.New()}

	if _, err := scanner.scanMovies(context.Background(), library, root); err == nil {
		t.Error("an unreadable directory was skipped, leaving its files out of the seen set")
	}
}
