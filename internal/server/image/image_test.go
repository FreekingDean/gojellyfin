package image

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/artwork"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type fixture struct {
	server    *Server
	client    *store.Client
	artwork   artwork.Store
	itemID    uuid.UUID
	directory string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	client := connection.Client()

	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	item, err := client.Item.Create().
		SetLibraryID(library.ID).
		SetKind(itemmodal.KindMovie).
		SetName("Movie").
		SetSortName("Movie").
		SetKey("test:movie").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the item: %v", err)
	}

	t.Cleanup(func() {
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	stored := artwork.New(client)

	return &fixture{
		server:    New(items.New(client), filesystem.New(env.Config{MediaDirectories: []string{filesystem.Root}}), stored),
		client:    client,
		artwork:   stored,
		itemID:    item.ID,
		directory: t.TempDir(),
	}
}

func (f *fixture) store(t *testing.T, kind items.ImageKind, index int32, key, tag string, content []byte) {
	t.Helper()

	ctx := context.Background()
	if err := f.artwork.Put(ctx, key, bytes.NewReader(content)); err != nil {
		t.Fatalf("failed to store %q: %v", key, err)
	}
	t.Cleanup(func() {
		if err := f.artwork.Delete(ctx, key); err != nil {
			t.Errorf("failed to clean up %q: %v", key, err)
		}
	})

	_, err := f.client.Image.Create().
		SetItemID(f.itemID).
		SetKind(kind).
		SetIndex(index).
		SetSource(imagemodal.SourceRemote).
		SetPath(key).
		SetTag(tag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the image row: %v", err)
	}
}

func (f *fixture) add(t *testing.T, kind items.ImageKind, index int32, name, tag string, content []byte) {
	t.Helper()

	path := filepath.Join(f.directory, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", name, err)
	}

	_, err := f.client.Image.Create().
		SetItemID(f.itemID).
		SetKind(kind).
		SetIndex(index).
		SetPath(path).
		SetTag(tag).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the image row: %v", err)
	}
}

func TestServer_GetItemImage(t *testing.T) {
	fixture := newFixture(t)
	poster := []byte("poster-bytes")

	t.Run("serves the file", func(t *testing.T) {
		fixture.add(t, imagemodal.KindPrimary, 0, "poster.jpg", "postertag", poster)

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.itemID,
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		recorder := httptest.NewRecorder()
		if err := response.VisitGetItemImageResponse(recorder); err != nil {
			t.Fatalf("failed to write the image: %v", err)
		}

		if recorder.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", recorder.Code)
		}
		if got := recorder.Body.String(); got != string(poster) {
			t.Errorf("body = %q, want %q", got, poster)
		}
		if got := recorder.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Errorf("content type = %q, want image/jpeg", got)
		}
		if got := recorder.Header().Get("Content-Length"); got != "12" {
			t.Errorf("content length = %q, want 12", got)
		}
	})

	t.Run("misses", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		fixture.add(t, imagemodal.KindPrimary, 0, "poster.jpg", "postertag", []byte("poster-bytes"))
		fixture.add(t, imagemodal.KindThumb, 0, "thumb.jpg", "thumbtag", []byte("thumb-bytes"))
		if err := os.Remove(filepath.Join(fixture.directory, "thumb.jpg")); err != nil {
			t.Fatalf("failed to remove the file: %v", err)
		}

		tests := []struct {
			name      string
			itemID    uuid.UUID
			imageType api.ImageType
		}{
			{name: "an image the item does not have", itemID: fixture.itemID, imageType: api.Logo},
			{name: "an unknown image type", itemID: fixture.itemID, imageType: api.ImageType("Nonsense")},
			{name: "an unknown item", itemID: uuid.New(), imageType: api.Primary},
			{name: "a row whose file is gone", itemID: fixture.itemID, imageType: api.Thumb},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response, err := fixture.server.GetItemImage(ctx, api.GetItemImageRequestObject{
					ItemId:    test.itemID,
					ImageType: test.imageType,
				})
				if err != nil {
					t.Fatalf("failed to get the image: %v", err)
				}

				if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
					t.Errorf("response = %T, want a 404", response)
				}
			})
		}
	})

}

func TestServer_GetItemImageByIndex(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	fixture.add(t, imagemodal.KindBackdrop, 0, "backdrop0.png", "first", []byte("first-backdrop"))
	fixture.add(t, imagemodal.KindBackdrop, 1, "backdrop1.png", "second", []byte("second-backdrop"))

	t.Run("serves the requested index", func(t *testing.T) {
		response, err := fixture.server.GetItemImageByIndex(ctx, api.GetItemImageByIndexRequestObject{
			ItemId:     fixture.itemID,
			ImageType:  api.Backdrop,
			ImageIndex: 1,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		recorder := httptest.NewRecorder()
		if err := response.VisitGetItemImageByIndexResponse(recorder); err != nil {
			t.Fatalf("failed to write the image: %v", err)
		}

		if got := recorder.Body.String(); got != "second-backdrop" {
			t.Errorf("body = %q, want second-backdrop", got)
		}
		if got := recorder.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("content type = %q, want image/png", got)
		}
	})

	t.Run("misses an index the item does not have", func(t *testing.T) {
		response, err := fixture.server.GetItemImageByIndex(ctx, api.GetItemImageByIndexRequestObject{
			ItemId:     fixture.itemID,
			ImageType:  api.Backdrop,
			ImageIndex: 7,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		if _, ok := response.(api.GetItemImageByIndex404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})
}

func TestServer_HeadItemImage(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	fixture.add(t, imagemodal.KindPrimary, 0, "poster.jpg", "postertag", []byte("poster-bytes"))

	t.Run("answers with the content headers", func(t *testing.T) {
		response, err := fixture.server.HeadItemImage(ctx, api.HeadItemImageRequestObject{
			ItemId:    fixture.itemID,
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to head the image: %v", err)
		}

		recorder := httptest.NewRecorder()
		if err := response.VisitHeadItemImageResponse(recorder); err != nil {
			t.Fatalf("failed to write the headers: %v", err)
		}

		if recorder.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", recorder.Code)
		}
		if got := recorder.Header().Get("Content-Length"); got != "12" {
			t.Errorf("content length = %q, want 12", got)
		}
		if got := recorder.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Errorf("content type = %q, want image/jpeg", got)
		}
	})

	t.Run("misses an image the item does not have", func(t *testing.T) {
		response, err := fixture.server.HeadItemImageByIndex(ctx, api.HeadItemImageByIndexRequestObject{
			ItemId:     fixture.itemID,
			ImageType:  api.Thumb,
			ImageIndex: 0,
		})
		if err != nil {
			t.Fatalf("failed to head the image: %v", err)
		}

		if _, ok := response.(api.HeadItemImageByIndex404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})
}

func TestServer_GetItemImage_stored(t *testing.T) {
	fixture := newFixture(t)
	poster := []byte("stored-poster-bytes")

	t.Run("serves the bytes the artwork store holds", func(t *testing.T) {
		fixture.store(t, imagemodal.KindPrimary, 0, "artwork/"+fixture.itemID.String()+"/Primary/0.jpg", "storedtag", poster)

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.itemID,
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		recorder := httptest.NewRecorder()
		if err := response.VisitGetItemImageResponse(recorder); err != nil {
			t.Fatalf("failed to write the image: %v", err)
		}

		if recorder.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", recorder.Code)
		}
		if got := recorder.Body.String(); got != string(poster) {
			t.Errorf("body = %q, want %q", got, poster)
		}
		if got := recorder.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Errorf("content type = %q, want the type the key's extension names", got)
		}
		if got := recorder.Header().Get("Content-Length"); got != "19" {
			t.Errorf("content length = %q, want 19", got)
		}
	})

	t.Run("misses a row whose bytes are gone", func(t *testing.T) {
		fixture := newFixture(t)
		key := "artwork/" + fixture.itemID.String() + "/Thumb/0.jpg"
		fixture.store(t, imagemodal.KindThumb, 0, key, "storedtag", poster)
		if err := fixture.artwork.Delete(context.Background(), key); err != nil {
			t.Fatalf("failed to delete the bytes: %v", err)
		}

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.itemID,
			ImageType: api.Thumb,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})
}
