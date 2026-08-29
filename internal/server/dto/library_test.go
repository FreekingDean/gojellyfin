package dto

import (
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

func TestLibraryView(t *testing.T) {
	t.Run("carries no image tag for a library with no collage", func(t *testing.T) {
		view := LibraryView(&libraries.Library{Name: "Movies"})

		if tags := *view.ImageTags; len(tags) != 0 {
			t.Errorf("image tags = %v, want none", tags)
		}
	})

	t.Run("carries the primary tag the client needs to ask for a collage", func(t *testing.T) {
		view := LibraryView(&libraries.Library{Name: "Movies", ImageTag: "abc123"})

		tag, ok := (*view.ImageTags)["Primary"]
		if !ok {
			t.Fatalf("image tags = %v, want a Primary entry", *view.ImageTags)
		}
		if *tag != "abc123" {
			t.Errorf("primary tag = %q, want %q", *tag, "abc123")
		}
	})
}
