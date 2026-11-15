package image

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const portrait = "https://image.tmdb.org/t/p/h632/portrait.jpg"

func TestServer_GetArtistImage(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	name := "Artist " + uuid.NewString()

	artist, err := fixture.client.Item.Create().
		SetKind(itemmodel.KindMusicArtist).
		SetName(name).
		SetSortName(name).
		SetKey("artist:mb:" + uuid.NewString()).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the artist: %v", err)
	}
	t.Cleanup(func() { _ = fixture.client.Item.DeleteOne(artist).Exec(ctx) })
	if err := fixture.client.LibraryItem.Create().
		SetLibraryID(fixture.libraryID).
		SetSourceID(fixture.sourceID).
		SetItemID(artist.ID).
		Exec(ctx); err != nil {
		t.Fatalf("failed to place the artist: %v", err)
	}
	if err := fixture.client.Image.Create().
		SetItemID(artist.ID).
		SetKind(imagemodel.KindPrimary).
		SetURL(portrait).
		SetTag("tag").
		Exec(ctx); err != nil {
		t.Fatalf("failed to create the image: %v", err)
	}

	response, err := fixture.server.GetArtistImage(ctx, api.GetArtistImageRequestObject{
		Name:      name,
		ImageType: api.Primary,
	})
	if err != nil {
		t.Fatalf("failed to get the image: %v", err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetArtistImageResponse(recorder); err != nil {
		t.Fatalf("failed to write the response: %v", err)
	}
	if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != portrait {
		t.Errorf("answered %d to %q, want 302 to %q", recorder.Code, recorder.Header().Get("Location"), portrait)
	}

	missing, err := fixture.server.GetArtistImage(ctx, api.GetArtistImageRequestObject{
		Name:      "Nobody " + uuid.NewString(),
		ImageType: api.Primary,
	})
	if err != nil {
		t.Fatalf("failed to ask for nobody: %v", err)
	}
	if _, ok := missing.(api.GetArtistImage404JSONResponse); !ok {
		t.Errorf("response = %T, want a 404", missing)
	}
}

func TestServer_GetPersonImage(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	name := "Person " + uuid.NewString()

	person, err := fixture.client.Person.Create().SetName(name).SetImageURL(portrait).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the person: %v", err)
	}
	t.Cleanup(func() { _ = fixture.client.Person.DeleteOne(person).Exec(ctx) })

	byName, err := fixture.server.GetPersonImage(ctx, api.GetPersonImageRequestObject{Name: name, ImageType: api.Primary})
	if err != nil {
		t.Fatalf("failed to get the image by name: %v", err)
	}
	recorder := httptest.NewRecorder()
	if err := byName.VisitGetPersonImageResponse(recorder); err != nil {
		t.Fatalf("failed to write the response: %v", err)
	}
	if recorder.Header().Get("Location") != portrait {
		t.Errorf("by name location = %q, want %q", recorder.Header().Get("Location"), portrait)
	}

	byID, err := fixture.server.GetItemImage(ctx, api.GetItemImageRequestObject{ItemId: person.ID, ImageType: api.Primary})
	if err != nil {
		t.Fatalf("failed to get the image by id: %v", err)
	}
	recorder = httptest.NewRecorder()
	if err := byID.VisitGetItemImageResponse(recorder); err != nil {
		t.Fatalf("failed to write the response: %v", err)
	}
	if recorder.Header().Get("Location") != portrait {
		t.Errorf("by id location = %q, want %q", recorder.Header().Get("Location"), portrait)
	}

	backdrop, err := fixture.server.GetPersonImage(ctx, api.GetPersonImageRequestObject{Name: name, ImageType: api.Backdrop})
	if err != nil {
		t.Fatalf("failed to ask for a backdrop: %v", err)
	}
	if _, ok := backdrop.(api.GetPersonImage404JSONResponse); !ok {
		t.Errorf("backdrop response = %T, want a 404", backdrop)
	}
}

func TestServer_GetGenreImage(t *testing.T) {
	var server Server

	response, err := server.GetGenreImage(context.Background(), api.GetGenreImageRequestObject{Name: "Drama", ImageType: api.Primary})
	if err != nil {
		t.Fatalf("failed to ask: %v", err)
	}
	if _, ok := response.(api.GetGenreImage404JSONResponse); !ok {
		t.Errorf("response = %T, want a 404", response)
	}
}
