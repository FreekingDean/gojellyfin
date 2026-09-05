package image

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/artwork"
	"github.com/FreekingDean/gojellyfin/internal/collage"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	downloadermodal "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type fixture struct {
	server     *Server
	client     *store.Client
	artwork    artwork.Store
	itemID     uuid.UUID
	libraryID  uuid.UUID
	directory  string
	downloader uuid.UUID
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

	downloader, err := client.Source.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(downloadermodal.KindRadarr).
		SetRootPath("/media").
		SetLocalPath("/media").
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	item, err := client.Item.Create().
		SetKind(itemmodal.KindMovie).
		SetName("Movie").
		SetSortName("Movie").
		SetKey("test:movie:" + library.ID.String()).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the item: %v", err)
	}

	if err := client.LibraryItem.Create().
		SetLibraryID(library.ID).
		SetSourceID(downloader.ID).
		SetItemID(item.ID).
		Exec(ctx); err != nil {
		t.Fatalf("failed to place the item in the library: %v", err)
	}

	t.Cleanup(func() {
		if err := client.Source.DeleteOne(downloader).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	stored := artwork.New(client)
	records := items.New(client)
	files := filesystem.New(env.Config{MediaDirectories: []string{filesystem.Root}})

	return &fixture{
		downloader: downloader.ID,
		server:     New(records, collage.New(records, files, stored), files, stored),
		client:     client,
		artwork:    stored,
		itemID:     item.ID,
		libraryID:  library.ID,
		directory:  t.TempDir(),
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
		SetPath(key).
		SetTag(tag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the image row: %v", err)
	}
}

func painted(t *testing.T) []byte {
	t.Helper()

	poster := image.NewRGBA(image.Rect(0, 0, 200, 300))
	for x := range 200 {
		for y := range 300 {
			poster.SetRGBA(x, y, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		}
	}

	written := &bytes.Buffer{}
	if err := png.Encode(written, poster); err != nil {
		t.Fatalf("failed to encode the poster: %v", err)
	}

	return written.Bytes()
}

func TestServer_GetItemImage(t *testing.T) {
	fixture := newFixture(t)
	poster := []byte("poster-bytes")

	t.Run("serves the file", func(t *testing.T) {
		fixture.store(t, imagemodal.KindPrimary, 0, "poster.jpg", "postertag", poster)

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

	t.Run("serves a library's collage under the library's own id", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		fixture.store(t, imagemodal.KindPrimary, 0, "items/"+fixture.itemID.String()+"/Primary/poster.png", "postertag", painted(t))

		response, err := fixture.server.GetItemImage(ctx, api.GetItemImageRequestObject{
			ItemId:    fixture.libraryID,
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
		if got := recorder.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Errorf("content type = %q, want image/jpeg", got)
		}
		if _, err := jpeg.Decode(recorder.Body); err != nil {
			t.Errorf("failed to decode the collage: %v", err)
		}
	})

	t.Run("misses", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		fixture.store(t, imagemodal.KindPrimary, 0, "poster.jpg", "postertag", []byte("poster-bytes"))
		if _, err := fixture.client.Image.Create().
			SetItemID(fixture.itemID).
			SetKind(imagemodal.KindThumb).
			SetIndex(0).
			SetPath("thumb.jpg").
			SetTag("thumbtag").
			Save(ctx); err != nil {
			t.Fatalf("failed to create the unstored image row: %v", err)
		}

		tests := []struct {
			name      string
			itemID    uuid.UUID
			imageType api.ImageType
		}{
			{name: "an image the item does not have", itemID: fixture.itemID, imageType: api.Logo},
			{name: "an unknown image type", itemID: fixture.itemID, imageType: api.ImageType("Nonsense")},
			{name: "an unknown item", itemID: uuid.New(), imageType: api.Primary},
			{name: "a row whose bytes are not stored", itemID: fixture.itemID, imageType: api.Thumb},
			{name: "a library with no collage", itemID: fixture.libraryID, imageType: api.Primary},
			{name: "an image type a library never has", itemID: fixture.libraryID, imageType: api.Thumb},
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

	fixture.store(t, imagemodal.KindBackdrop, 0, "backdrop0.png", "first", []byte("first-backdrop"))
	fixture.store(t, imagemodal.KindBackdrop, 1, "backdrop1.png", "second", []byte("second-backdrop"))

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

	fixture.store(t, imagemodal.KindPrimary, 0, "poster.jpg", "postertag", []byte("poster-bytes"))

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
