package items

import (
	"context"
	"testing"

	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_SaveImage(t *testing.T) {
	t.Run("keeps the first poster the scan found", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		movie := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Dune"})

		for _, path := range []string{"/media/poster.jpg", "/media/folder.jpg"} {
			artwork := Artwork{Kind: imagemodal.KindPrimary, Path: path, Tag: path}
			if err := fixture.service.SaveImage(ctx, movie, artwork); err != nil {
				t.Fatalf("failed to save %q: %v", path, err)
			}
		}

		record, err := fixture.service.Image(ctx, movie, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatalf("failed to read the image back: %v", err)
		}
		if record.Path != "/media/poster.jpg" {
			t.Errorf("path = %q, want the first find kept", record.Path)
		}
	})

	t.Run("displaces a poster the identify job downloaded", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		movie := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Dune"})

		downloaded := Artwork{Kind: imagemodal.KindPrimary, Path: "items/dune/Primary/abc.jpg", Tag: "abc"}
		if err := fixture.service.SaveDownloadedImage(ctx, movie, downloaded); err != nil {
			t.Fatalf("failed to save the downloaded poster: %v", err)
		}

		found := Artwork{Kind: imagemodal.KindPrimary, Path: "/media/poster.jpg", Tag: "onwards"}
		if err := fixture.service.SaveImage(ctx, movie, found); err != nil {
			t.Fatalf("failed to save the scanned poster: %v", err)
		}

		record, err := fixture.service.Image(ctx, movie, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatalf("failed to read the image back: %v", err)
		}
		if record.Source != imagemodal.SourceLocal || record.Path != "/media/poster.jpg" {
			t.Errorf("image = %s %q, want the poster beside the file to win", record.Source, record.Path)
		}
		if record.Tag != "onwards" {
			t.Errorf("tag = %q, want the displacing poster's own tag", record.Tag)
		}
	})
}

func TestService_SaveDownloadedImage(t *testing.T) {
	t.Run("gives way to a poster the scan already found", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		movie := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Dune"})

		found := Artwork{Kind: imagemodal.KindPrimary, Path: "/media/poster.jpg", Tag: "local"}
		if err := fixture.service.SaveImage(ctx, movie, found); err != nil {
			t.Fatalf("failed to save the scanned poster: %v", err)
		}

		downloaded := Artwork{Kind: imagemodal.KindPrimary, Path: "items/dune/Primary/abc.jpg", Tag: "abc"}
		if err := fixture.service.SaveDownloadedImage(ctx, movie, downloaded); err != nil {
			t.Fatalf("failed to save the downloaded poster: %v", err)
		}

		record, err := fixture.service.Image(ctx, movie, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatalf("failed to read the image back: %v", err)
		}
		if record.Source != imagemodal.SourceLocal || record.Path != "/media/poster.jpg" {
			t.Errorf("image = %s %q, want the poster beside the file to win", record.Source, record.Path)
		}
	})

	t.Run("replaces the poster it wrote before", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		movie := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Dune"})

		for _, tag := range []string{"abc", "def"} {
			downloaded := Artwork{Kind: imagemodal.KindPrimary, Path: "items/dune/Primary/" + tag + ".jpg", Tag: tag}
			if err := fixture.service.SaveDownloadedImage(ctx, movie, downloaded); err != nil {
				t.Fatalf("failed to save %q: %v", tag, err)
			}
		}

		record, err := fixture.service.Image(ctx, movie, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatalf("failed to read the image back: %v", err)
		}
		if record.Path != "items/dune/Primary/def.jpg" {
			t.Errorf("path = %q, want the newer poster", record.Path)
		}
		if record.Tag != "def" {
			t.Errorf("tag = %q, want a changed poster to bust the client's cache", record.Tag)
		}
	})
}
