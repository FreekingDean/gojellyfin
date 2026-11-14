package dto

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

type collageTag string

func (c collageTag) Tag(context.Context, uuid.UUID) (string, error) {
	return string(c), nil
}

func TestLibraryView(t *testing.T) {
	t.Run("carries no image tag when the library has no posters, so the client asks for no picture", func(t *testing.T) {
		view, err := LibraryView(context.Background(), collageTag(""), &libraries.Library{Name: "Movies"})
		if err != nil {
			t.Fatalf("failed to build the view: %v", err)
		}

		if len(*view.ImageTags) != 0 {
			t.Errorf("image tags = %v, want none", *view.ImageTags)
		}
	})

	t.Run("names the collage as its primary image when the library has posters", func(t *testing.T) {
		view, err := LibraryView(context.Background(), collageTag("abc"), &libraries.Library{Name: "Movies"})
		if err != nil {
			t.Fatalf("failed to build the view: %v", err)
		}

		tag := (*view.ImageTags)["Primary"]
		if tag == nil || *tag != "abc" {
			t.Errorf("image tags = %v, want Primary=abc", *view.ImageTags)
		}
	})
}
