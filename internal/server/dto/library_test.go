package dto

import (
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

func TestLibraryView(t *testing.T) {
	t.Run("carries no image tag, so the client asks for no picture", func(t *testing.T) {
		view := LibraryView(&libraries.Library{Name: "Movies"})

		if len(*view.ImageTags) != 0 {
			t.Errorf("image tags = %v, want none", *view.ImageTags)
		}
	})
}
