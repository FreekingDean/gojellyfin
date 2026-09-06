package items

import (
	"context"
	"testing"

	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_SaveImage(t *testing.T) {
	t.Run("replaces the poster it wrote before", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		movie := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Dune"})

		for _, tag := range []string{"abc", "def"} {
			poster := Image{
				Kind: imagemodal.KindPrimary,
				URL:  "https://image.tmdb.org/t/p/w780/" + tag + ".jpg",
				Tag:  tag,
			}
			if err := fixture.service.SaveImage(ctx, movie, poster); err != nil {
				t.Fatalf("failed to save %q: %v", tag, err)
			}
		}

		record, err := fixture.service.Image(ctx, movie, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatalf("failed to read the image back: %v", err)
		}
		if record.URL != "https://image.tmdb.org/t/p/w780/def.jpg" {
			t.Errorf("url = %q, want the newer poster", record.URL)
		}
		if record.Tag != "def" {
			t.Errorf("tag = %q, want a changed poster to bust the client's cache", record.Tag)
		}
	})
}
