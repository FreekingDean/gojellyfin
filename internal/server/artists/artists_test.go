package artists

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type fixture struct {
	server    *Server
	service   *items.Service
	libraryID uuid.UUID
	artist    string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	ctx := context.Background()

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

	client := connection.Client()
	name := t.Name() + "-" + uuid.NewString()
	library, err := client.Library.Create().SetName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := items.New(client)
	fixture := &fixture{
		server:    New(service, libraries.New(client)),
		service:   service,
		libraryID: library.ID,
		artist:    name,
	}

	artist := fixture.save(t, itemmodal.KindMusicArtist, name, nil)
	album := fixture.save(t, itemmodal.KindMusicAlbum, name+" Album", &artist.ID)
	fixture.save(t, itemmodal.KindAudio, name+" Track", &album.ID)
	fixture.save(t, itemmodal.KindMusicArtist, name+" Singleton", nil)

	return fixture
}

func (f *fixture) save(t *testing.T, kind items.Kind, name string, parentID *uuid.UUID) *items.Item {
	t.Helper()

	record, err := f.service.SaveScanned(context.Background(), items.Scanned{
		LibraryID: f.libraryID,
		ParentID:  parentID,
		Kind:      kind,
		Name:      name,
		SortName:  name,
		Key:       "test:" + name,
	})
	if err != nil {
		t.Fatalf("failed to save %q: %v", name, err)
	}

	return record
}

func TestServer_GetArtists(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.server.GetArtists(context.Background(), api.GetArtistsRequestObject{
		Params: api.GetArtistsParams{ParentId: &fixture.libraryID},
	})
	if err != nil {
		t.Fatalf("failed to get the artists: %v", err)
	}

	result, ok := response.(api.GetArtists200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetArtists200JSONResponse", response)
	}
	if result.TotalRecordCount == nil || *result.TotalRecordCount != 2 {
		t.Fatalf("total = %v, want 2", result.TotalRecordCount)
	}

	first := (*result.Items)[0]
	if first.Type == nil || *first.Type != api.BaseItemKindMusicArtist {
		t.Errorf("type = %v, want MusicArtist", first.Type)
	}
	if first.IsFolder == nil || !*first.IsFolder {
		t.Error("an artist is a folder")
	}
}

func TestServer_GetAlbumArtists(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.server.GetAlbumArtists(context.Background(), api.GetAlbumArtistsRequestObject{
		Params: api.GetAlbumArtistsParams{ParentId: &fixture.libraryID},
	})
	if err != nil {
		t.Fatalf("failed to get the album artists: %v", err)
	}

	result, ok := response.(api.GetAlbumArtists200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetAlbumArtists200JSONResponse", response)
	}
	if result.TotalRecordCount == nil || *result.TotalRecordCount != 1 {
		t.Fatalf("total = %v, want 1", result.TotalRecordCount)
	}
	if name := (*result.Items)[0].Name; name == nil || *name != fixture.artist {
		t.Errorf("name = %v, want %s", name, fixture.artist)
	}
}

func TestServer_GetArtistByName(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.server.GetArtistByName(context.Background(), api.GetArtistByNameRequestObject{
		Name: fixture.artist,
	})
	if err != nil {
		t.Fatalf("failed to get the artist: %v", err)
	}

	result, ok := response.(api.GetArtistByName200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetArtistByName200JSONResponse", response)
	}
	if result.Name == nil || *result.Name != fixture.artist {
		t.Errorf("name = %v, want %s", result.Name, fixture.artist)
	}
	if result.ChildCount == nil || *result.ChildCount != 1 {
		t.Errorf("child count = %v, want 1", result.ChildCount)
	}
}
