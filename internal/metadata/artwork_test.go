package metadata

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/blob"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func cdn(t *testing.T, body string) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server.URL + "/poster.jpg"
}

func TestService_CacheArtwork(t *testing.T) {
	t.Run("stores the bytes the provider pointed at", func(t *testing.T) {
		fixture := newFixture(t)
		added := fixture.add(t, items.Item{Name: "Cached Poster", Kind: itemmodal.KindMovie})
		url := cdn(t, "poster-bytes")

		image := items.Image{Kind: imagemodal.KindPrimary, URL: url, Tag: tag(url)}
		if err := fixture.items.SaveImage(context.Background(), added.ID, image); err != nil {
			t.Fatalf("failed to save the image row: %v", err)
		}

		if err := fixture.service.CacheArtwork(context.Background(), fixture.libraryID); err != nil {
			t.Fatalf("failed to cache artwork: %v", err)
		}

		stored, err := fixture.items.Image(context.Background(), added.ID, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatal(err)
		}

		want := items.ImageKey(added.ID, imagemodal.KindPrimary, 0)
		if stored.Key != want {
			t.Fatalf("key = %q, want %q", stored.Key, want)
		}

		object, err := fixture.objects.Get(context.Background(), want)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = object.Body.Close() }()

		read, err := io.ReadAll(object.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(read) != "poster-bytes" {
			t.Errorf("body = %q, want poster-bytes", read)
		}
		if object.ContentType != "image/jpeg" {
			t.Errorf("content type = %q, want image/jpeg", object.ContentType)
		}
	})

	t.Run("leaves a row alone once it names a key", func(t *testing.T) {
		fixture := newFixture(t)
		added := fixture.add(t, items.Item{Name: "Already Cached", Kind: itemmodal.KindMovie})
		url := cdn(t, "poster-bytes")

		image := items.Image{Kind: imagemodal.KindPrimary, URL: url, Tag: tag(url)}
		if err := fixture.items.SaveImage(context.Background(), added.ID, image); err != nil {
			t.Fatalf("failed to save the image row: %v", err)
		}
		if err := fixture.service.CacheArtwork(context.Background(), fixture.libraryID); err != nil {
			t.Fatalf("failed to cache artwork: %v", err)
		}

		pending, err := fixture.items.ImagesNeedingCache(context.Background(), fixture.libraryID, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, image := range pending {
			if image.ItemID == added.ID {
				t.Error("the cached image is still pending")
			}
		}
	})

	t.Run("keeps the key while the url is unchanged and drops it when it moves", func(t *testing.T) {
		fixture := newFixture(t)
		added := fixture.add(t, items.Item{Name: "Replaced Poster", Kind: itemmodal.KindMovie})
		url := cdn(t, "poster-bytes")

		image := items.Image{Kind: imagemodal.KindPrimary, URL: url, Tag: tag(url)}
		if err := fixture.items.SaveImage(context.Background(), added.ID, image); err != nil {
			t.Fatalf("failed to save the image row: %v", err)
		}
		if err := fixture.service.CacheArtwork(context.Background(), fixture.libraryID); err != nil {
			t.Fatalf("failed to cache artwork: %v", err)
		}

		if err := fixture.items.SaveImage(context.Background(), added.ID, image); err != nil {
			t.Fatalf("failed to re-save the image row: %v", err)
		}

		stored, err := fixture.items.Image(context.Background(), added.ID, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Key == "" {
			t.Error("an unchanged url lost its key")
		}

		moved := cdn(t, "other-bytes")
		if err := fixture.items.SaveImage(
			context.Background(),
			added.ID,
			items.Image{Kind: imagemodal.KindPrimary, URL: moved, Tag: tag(moved)},
		); err != nil {
			t.Fatalf("failed to save the moved image row: %v", err)
		}

		stored, err = fixture.items.Image(context.Background(), added.ID, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Key != "" {
			t.Errorf("key = %q, want it cleared by the new url", stored.Key)
		}
	})

	t.Run("does nothing without an object store", func(t *testing.T) {
		fixture := newFixture(t)
		added := fixture.add(t, items.Item{Name: "No Object Store", Kind: itemmodal.KindMovie})

		disabled, err := blob.New(env.Config{})
		if err != nil {
			t.Fatal(err)
		}
		service := New(fixture.provider, fixture.items, disabled)
		url := cdn(t, "poster-bytes")

		image := items.Image{Kind: imagemodal.KindPrimary, URL: url, Tag: tag(url)}
		if err := fixture.items.SaveImage(context.Background(), added.ID, image); err != nil {
			t.Fatalf("failed to save the image row: %v", err)
		}

		if err := service.CacheArtwork(context.Background(), fixture.libraryID); err != nil {
			t.Fatalf("failed to cache artwork: %v", err)
		}

		stored, err := fixture.items.Image(context.Background(), added.ID, imagemodal.KindPrimary, 0)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Key != "" {
			t.Errorf("key = %q, want nothing stored", stored.Key)
		}
	})
}
