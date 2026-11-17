package genres

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	genremodel "github.com/FreekingDean/gojellyfin/internal/store/genre"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
	downloadermodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func TestServer_GetGenres(t *testing.T) {
	ctx := context.Background()

	connection := storetest.Open(t)

	client := connection.Client()
	name := t.Name() + "-" + uuid.NewString()
	library, err := client.Library.Create().SetName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(serverGetGenresCleanup(t, library, client, ctx, name))

	downloader, err := client.Source.Create().
		SetName(name).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(downloadermodel.KindRadarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Source.DeleteOne(downloader).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	service := items.New(client)
	movie, err := service.SaveScanned(ctx, items.Item{
		Kind:     itemmodel.KindMovie,
		Name:     name,
		SortName: name,
		Key:      "test:" + name,
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	serverGetGenresRest(t, service, ctx, library, downloader, movie, name, err)
}

func serverGetGenresRest(t *testing.T, service *items.Service, ctx context.Context, library *store.Library, downloader *store.Source, movie *items.Item, name string, err error) {
	t.Helper()

	if err := service.SaveMembership(ctx, library.ID, downloader.ID, []uuid.UUID{movie.ID}); err != nil {
		t.Fatalf("failed to place the item in the library: %v", err)
	}
	if _, err := service.UpdateMetadata(ctx, movie.ID, items.Metadata{Genres: &[]string{name}}); err != nil {
		t.Fatalf("failed to save the probe: %v", err)
	}

	hidden, err := New(service, access{}).GetGenres(ctx, api.GetGenresRequestObject{
		Params: api.GetGenresParams{ParentId: &library.ID},
	})
	if err != nil {
		t.Fatalf("failed to get the genres: %v", err)
	}
	if listed := hidden.(api.GetGenres200JSONResponse); len(*listed.Items) != 0 {
		t.Errorf("genres = %d, want none for a user who cannot see the library", len(*listed.Items))
	}

	response, err := New(service, access{All: true}).GetGenres(ctx, api.GetGenresRequestObject{
		Params: api.GetGenresParams{ParentId: &library.ID},
	})
	if err != nil {
		t.Fatalf("failed to get the genres: %v", err)
	}

	result, ok := response.(api.GetGenres200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetGenres200JSONResponse", response)
	}
	if result.TotalRecordCount == nil || *result.TotalRecordCount != 1 {
		t.Fatalf("total = %v, want 1", result.TotalRecordCount)
	}

	dto := (*result.Items)[0]
	if dto.Name == nil || *dto.Name != name {
		t.Errorf("name = %v, want %s", dto.Name, name)
	}
	if dto.Type == nil || *dto.Type != api.BaseItemKindGenre {
		t.Errorf("type = %v, want %s", dto.Type, api.BaseItemKindGenre)
	}
	if dto.Id == nil || *dto.Id == uuid.Nil {
		t.Errorf("id = %v, want a genre id", dto.Id)
	}
}

func serverGetGenresCleanup(t *testing.T, library *store.Library, client *store.Client, ctx context.Context, name string) func() {
	t.Helper()

	return func() {
		owned := itemmodel.HasLibrariesWith(librarymembership.LibraryID(library.ID))
		if _, err := client.ItemSource.Delete().Where(sourcemodel.HasItemWith(owned)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if _, err := client.Item.Delete().Where(owned).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if _, err := client.Genre.Delete().Where(genremodel.Name(name)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the genre: %v", err)
		}
	}
}

type access users.Access

func (a access) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access(a), nil
}
