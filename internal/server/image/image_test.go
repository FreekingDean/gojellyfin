package image

import (
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/cards"
	settings "github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	downloadermodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	server    *Server
	client    *store.Client
	itemID    uuid.UUID
	libraryID uuid.UUID
	sourceID  uuid.UUID
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
		SetKind(downloadermodel.KindRadarr).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	item, err := client.Item.Create().
		SetKind(itemmodel.KindMovie).
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

	catalogue := items.New(client)
	found := discover.New(trendingProvider{}, catalogue)
	shelved := shelves.New(catalogue, found, settings.New(client))

	return &fixture{
		server:    New(catalogue, cards.New(catalogue, libraries.New(client), shelved, users.New(client)), found, nil),
		client:    client,
		itemID:    item.ID,
		libraryID: library.ID,
		sourceID:  downloader.ID,
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

const (
	unownedPoster   = "https://image.tmdb.org/t/p/w780/unowned.jpg"
	unownedBackdrop = "https://image.tmdb.org/t/p/w1280/unowned.jpg"
)

type trendingProvider struct{}

func (trendingProvider) Enabled() bool { return true }

func (trendingProvider) Movie(context.Context, int) (items.Metadata, bool, error) {
	return items.Metadata{Images: []items.RemoteImage{
		{Kind: imagemodel.KindPrimary, URL: unownedPoster},
		{Kind: imagemodel.KindBackdrop, URL: unownedBackdrop},
	}}, true, nil
}

func (trendingProvider) Series(context.Context, int) (items.Metadata, bool, error) {
	return items.Metadata{}, true, nil
}

func (trendingProvider) Trending(context.Context, items.Kind, int) ([]items.Metadata, error) {
	return nil, nil
}

func (trendingProvider) Search(context.Context, items.Kind, string) ([]items.Metadata, error) {
	return nil, nil
}

const poster = "https://image.tmdb.org/t/p/w780/poster.jpg"

func TestServer_GetItemImage(t *testing.T) {
	t.Run("redirects to the url the provider answered", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store(t, imagemodel.KindPrimary, 0, poster, "postertag")

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
		fixture.store(t, imagemodel.KindPrimary, 0, poster, "postertag")

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

func TestServer_GetItemImage_Unowned(t *testing.T) {
	fixture := newFixture(t)

	tests := []struct {
		name      string
		itemID    uuid.UUID
		imageType api.ImageType
		want      string
	}{
		{name: "the poster of a title the library lacks", itemID: discover.ID(itemmodel.KindMovie, 603), imageType: api.Primary, want: unownedPoster},
		{name: "its backdrop", itemID: discover.ID(itemmodel.KindMovie, 603), imageType: api.Backdrop, want: unownedBackdrop},
		{name: "a kind the provider sent no picture for", itemID: discover.ID(itemmodel.KindMovie, 603), imageType: api.Logo},
		{name: "a title with no artwork", itemID: discover.ID(itemmodel.KindSeries, 1396), imageType: api.Primary},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
				ItemId:    test.itemID,
				ImageType: test.imageType,
			})
			if err != nil {
				t.Fatalf("failed to get the image: %v", err)
			}

			if test.want == "" {
				if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
					t.Errorf("response = %T, want a 404", response)
				}
				return
			}

			recorder := httptest.NewRecorder()
			if err := response.VisitGetItemImageResponse(recorder); err != nil {
				t.Fatalf("failed to write the response: %v", err)
			}
			if got := recorder.Header().Get("Location"); recorder.Code != http.StatusFound || got != test.want {
				t.Errorf("answered %d to %q, want a 302 to %q", recorder.Code, got, test.want)
			}
		})
	}
}

func TestServer_GetItemImageByIndex(t *testing.T) {
	fixture := newFixture(t)
	second := "https://image.tmdb.org/t/p/w1280/backdrop.jpg"
	fixture.store(t, imagemodel.KindBackdrop, 1, second, "backdroptag")

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

func TestServer_GetItemImage_Library(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)

			return
		}
		poster := image.NewRGBA(image.Rect(0, 0, 20, 30))
		_ = png.Encode(w, poster)
	}))
	t.Cleanup(cdn.Close)

	t.Run("serves a card of the library's posters", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store(t, imagemodel.KindPrimary, 0, cdn.URL+"/poster.png", "postertag")

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.libraryID,
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		recorder := httptest.NewRecorder()
		if err := response.VisitGetItemImageResponse(recorder); err != nil {
			t.Fatalf("failed to write the response: %v", err)
		}

		if recorder.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", recorder.Code)
		}
		if got := recorder.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Errorf("content type = %q, want image/jpeg", got)
		}
		if got := recorder.Header().Get("Cache-Control"); got != cacheFor {
			t.Errorf("cache control = %q, want %q", got, cacheFor)
		}
		if _, _, err := image.Decode(recorder.Body); err != nil {
			t.Errorf("failed to decode the card: %v", err)
		}
	})

	t.Run("tells the client not to keep a card missing a poster", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store(t, imagemodel.KindPrimary, 0, cdn.URL+"/poster.png", "postertag")

		other, err := fixture.client.Item.Create().
			SetKind(itemmodel.KindMovie).
			SetName("Other").
			SetSortName("Other").
			SetKey("test:other:" + fixture.libraryID.String()).
			Save(context.Background())
		if err != nil {
			t.Fatalf("failed to create the item: %v", err)
		}
		t.Cleanup(func() { _ = fixture.client.Item.DeleteOne(other).Exec(context.Background()) })
		if err := fixture.client.LibraryItem.Create().
			SetLibraryID(fixture.libraryID).
			SetSourceID(fixture.sourceID).
			SetItemID(other.ID).
			Exec(context.Background()); err != nil {
			t.Fatalf("failed to place the item in the library: %v", err)
		}
		if _, err := fixture.client.Image.Create().
			SetItemID(other.ID).
			SetKind(imagemodel.KindPrimary).
			SetURL(cdn.URL + "/missing").
			SetTag("missingtag").
			Save(context.Background()); err != nil {
			t.Fatalf("failed to create the image row: %v", err)
		}

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.libraryID,
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		recorder := httptest.NewRecorder()
		if err := response.VisitGetItemImageResponse(recorder); err != nil {
			t.Fatalf("failed to write the response: %v", err)
		}

		if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("cache control = %q, want no-store", got)
		}
	})

	t.Run("answers 404 for a library with no posters", func(t *testing.T) {
		fixture := newFixture(t)

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.libraryID,
			ImageType: api.Primary,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})

	t.Run("answers 404 for a library image that is not its primary", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store(t, imagemodel.KindPrimary, 0, cdn.URL+"/poster.png", "postertag")

		response, err := fixture.server.GetItemImage(context.Background(), api.GetItemImageRequestObject{
			ItemId:    fixture.libraryID,
			ImageType: api.Backdrop,
		})
		if err != nil {
			t.Fatalf("failed to get the image: %v", err)
		}

		if _, ok := response.(api.GetItemImage404JSONResponse); !ok {
			t.Errorf("response = %T, want a 404", response)
		}
	})
}
