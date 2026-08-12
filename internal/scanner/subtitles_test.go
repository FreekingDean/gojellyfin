package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
)

func TestSubtitlesBeside(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{
		"Blade Runner (1982).mkv",
		"Blade Runner (1982).en.srt",
		"Blade Runner (1982).fr.forced.srt",
		"Blade Runner (1982).jpg",
		"Some Other Movie.en.srt",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("failed to write %q: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(directory, "Blade Runner (1982).de.srt"), 0o700); err != nil {
		t.Fatalf("failed to create the directory: %v", err)
	}

	found, err := subtitlesBeside(context.Background(), filesystem.New(), filepath.Join(directory, "Blade Runner (1982).mkv"))
	if err != nil {
		t.Fatalf("failed to discover the subtitles: %v", err)
	}

	if len(found) != 2 {
		t.Fatalf("subtitles = %d, want 2", len(found))
	}
	if found[0].Language != "en" || found[1].Language != "fr" {
		t.Errorf("languages = %q, %q, want en, fr", found[0].Language, found[1].Language)
	}
	if !found[1].IsForced {
		t.Error("the fr track should be forced")
	}
	if want := filepath.Join(directory, "Blade Runner (1982).en.srt"); found[0].Path != want {
		t.Errorf("path = %q, want %q", found[0].Path, want)
	}
}

func TestSubtitlesBesideMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone", "Movie.mkv")

	if _, err := subtitlesBeside(context.Background(), filesystem.New(), path); err == nil {
		t.Error("want an error for a directory that is not there")
	}
}
