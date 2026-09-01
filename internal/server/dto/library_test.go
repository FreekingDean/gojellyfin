package dto

import (
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/collage"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

func TestLibraryView(t *testing.T) {
	t.Run("carries the primary tag the client needs to ask for a collage", func(t *testing.T) {
		view := LibraryView(&libraries.Library{Name: "Movies"})

		tag, ok := (*view.ImageTags)["Primary"]
		if !ok {
			t.Fatalf("image tags = %v, want a Primary entry", *view.ImageTags)
		}
		if *tag != collage.Tag {
			t.Errorf("primary tag = %q, want %q", *tag, collage.Tag)
		}
	})
}
