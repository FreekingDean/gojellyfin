package artists

import (
	"context"
	"errors"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type allLibraries struct{}

func (allLibraries) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access{All: true}, nil
}

type fixture struct {
	server  *Server
	library uuid.UUID
	client  *store.Client
	source  uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection := storetest.Open(t)

	ctx := context.Background()
	client := connection.Client()
	collections := libraries.New(client)

	library, err := collections.CreateLibrary(ctx, t.Name()+"-"+uuid.NewString(), librarymodel.CollectionTypeMusic, nil)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	downloader, err := client.Source.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind("lidarr").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}

	t.Cleanup(func() {
		if err := collections.DeleteLibrary(ctx, library.ID); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := client.Source.DeleteOneID(downloader.ID).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	return &fixture{
		server:  New(items.New(client), collections, allLibraries{}),
		library: library.ID,
		client:  client,
		source:  downloader.ID,
	}
}

func (f *fixture) add(t *testing.T, kind items.Kind, name string, parent *uuid.UUID) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	service := f.server.items
	item, err := service.SaveScanned(ctx, items.Item{
		ParentID: parent,
		Kind:     kind,
		Key:      string(kind) + ":mb:" + uuid.NewString(),
		Name:     name,
		SortName: items.SortName(name),
	})
	if err != nil {
		t.Fatalf("failed to save %s: %v", name, err)
	}
	if err := service.SaveMembership(ctx, f.library, f.source, []uuid.UUID{item.ID}); err != nil {
		t.Fatalf("failed to put %s in the library: %v", name, err)
	}

	return item.ID
}

func TestServer_GetAlbumArtists(t *testing.T) {
	fixture := newFixture(t)
	band := fixture.add(t, itemmodel.KindMusicArtist, "The Band", nil)
	fixture.add(t, itemmodel.KindMusicArtist, "Another", nil)
	fixture.add(t, itemmodel.KindMusicAlbum, "Album", &band)

	response, err := fixture.server.GetAlbumArtists(context.Background(), api.GetAlbumArtistsRequestObject{
		Params: api.GetAlbumArtistsParams{ParentId: &fixture.library},
	})
	if err != nil {
		t.Fatalf("failed to list the artists: %v", err)
	}

	result := api.BaseItemDtoQueryResult(response.(api.GetAlbumArtists200JSONResponse))
	if got := apiutil.Deref(result.TotalRecordCount); got != 2 {
		t.Fatalf("total = %d, want the two artists and not the album", got)
	}
	names := []string{apiutil.Deref((*result.Items)[0].Name), apiutil.Deref((*result.Items)[1].Name)}
	if names[0] != "Another" || names[1] != "The Band" {
		t.Errorf("artists = %v, want them in sort name order with the article dropped", names)
	}
}

func TestServer_GetArtistByName(t *testing.T) {
	fixture := newFixture(t)
	band := fixture.add(t, itemmodel.KindMusicArtist, "The Band "+uuid.NewString(), nil)

	named, err := fixture.server.items.ItemByID(context.Background(), items.Everyone, band)
	if err != nil {
		t.Fatalf("failed to read the artist back: %v", err)
	}

	response, err := fixture.server.GetArtistByName(context.Background(), api.GetArtistByNameRequestObject{Name: named.Name})
	if err != nil {
		t.Fatalf("failed to find the artist: %v", err)
	}
	if got := api.BaseItemDto(response.(api.GetArtistByName200JSONResponse)); apiutil.Deref(got.Id) != band {
		t.Errorf("answered %v, want the artist named", got.Id)
	}

	_, err = fixture.server.GetArtistByName(context.Background(), api.GetArtistByNameRequestObject{Name: uuid.NewString()})
	if !errors.Is(err, items.ErrNotFound) {
		t.Errorf("err = %v, want %v for a name nobody holds", err, items.ErrNotFound)
	}
}
