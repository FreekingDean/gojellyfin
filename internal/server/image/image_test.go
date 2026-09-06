package image

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	downloadermodal "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type fixture struct {
	server *Server
	client *store.Client
	itemID uuid.UUID
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

	return &fixture{
		server: New(items.New(client)),
		client: client,
		itemID: item.ID,
	}
}

func (f *fixture) store(t *testing.T, kind items.ImageKind, index int32, url, tag string) {
	t.Helper()

	_, err := f.client.Image.Create().
		SetItemID(f.itemID).
		SetKind(kind).
		SetIndex(index).
		SetURL(url).
		SetTag(tag).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the image row: %v", err)
	}
}

const poster = "https://image.tmdb.org/t/p/w780/poster.jpg"

func TestServer_GetItemImage(t *testing.T) {
	t.Run("redirects to the url the provider answered", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store(t, imagemodal.KindPrimary, 0, poster, "postertag")

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.itemID,
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		recorder := httptest.NewRecorder()
		if err := response.VisitGetItemImageResponse(recorder); err != nil {
			t.Fatalf("failed to write the response: %v", err)
		}

		if recorder.Code != http.StatusFound {
			t.Errorf("status = %d, want 302", recorder.Code)
		}
		if got := recorder.Header().Get("Location"); got != poster {
			t.Errorf("location = %q, want %q", got, poster)
		}
	})

	t.Run("answers 404 for an item with no image", func(t *testing.T) {
		fixture := newFixture(t)

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.itemID,
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})

	t.Run("answers 404 for an image type nothing wrote", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store(t, imagemodal.KindPrimary, 0, poster, "postertag")

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
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
}

func TestServer_GetItemImageByIndex(t *testing.T) {
	fixture := newFixture(t)
	second := "https://image.tmdb.org/t/p/w1280/backdrop.jpg"
	fixture.store(t, imagemodal.KindBackdrop, 1, second, "backdroptag")

	response, err := fixture.server.GetItemImageByIndex(context.Background(), api.GetItemImageByIndexRequestObject{
		ItemId:     fixture.itemID,
		ImageType:  api.Backdrop,
		ImageIndex: 1,
	})
	if err != nil {
		t.Fatalf("failed to get the image: %v", err)
	}

	recorder := httptest.NewRecorder()
	if err := response.VisitGetItemImageByIndexResponse(recorder); err != nil {
		t.Fatalf("failed to write the response: %v", err)
	}

	if got := recorder.Header().Get("Location"); got != second {
		t.Errorf("location = %q, want %q", got, second)
	}
}
