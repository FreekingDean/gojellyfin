package image

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

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
	itemID    uuid.UUID
	directory string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
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

	return &fixture{
		server:    New(items.New(client), filesystem.New()),
		client:    client,
		itemID:    item.ID,
		directory: t.TempDir(),
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

func TestGetItemImage(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	poster := []byte("poster-bytes")

	fixture.add(t, imagemodal.KindPrimary, 0, "poster.jpg", "postertag", poster)

	t.Run("serves the file", func(t *testing.T) {
		response, err := fixture.server.GetItemImage(ctx, api.GetItemImageRequestObject{
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

	t.Run("misses an image the item does not have", func(t *testing.T) {
		response, err := fixture.server.GetItemImage(ctx, api.GetItemImageRequestObject{
			ItemId:    fixture.itemID,
			ImageType: api.Logo,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})

	t.Run("misses an unknown image type", func(t *testing.T) {
		response, err := fixture.server.GetItemImage(ctx, api.GetItemImageRequestObject{
			ItemId:    fixture.itemID,
			ImageType: api.ImageType("Nonsense"),
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})

	t.Run("misses an unknown item", func(t *testing.T) {
		response, err := fixture.server.GetItemImage(ctx, api.GetItemImageRequestObject{
			ItemId:    uuid.New(),
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})

	t.Run("misses a row whose file is gone", func(t *testing.T) {
		fixture.add(t, imagemodal.KindThumb, 0, "thumb.jpg", "thumbtag", []byte("thumb-bytes"))
		if err := os.Remove(filepath.Join(fixture.directory, "thumb.jpg")); err != nil {
			t.Fatalf("failed to remove the file: %v", err)
		}

		response, err := fixture.server.GetItemImage(ctx, api.GetItemImageRequestObject{
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

func TestGetItemImageByIndex(t *testing.T) {
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

func TestHeadItemImage(t *testing.T) {
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
