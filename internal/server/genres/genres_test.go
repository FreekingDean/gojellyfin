package genres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	genremodal "github.com/FreekingDean/gojellyfin/internal/store/genre"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
)

func TestServer_GetGenres(t *testing.T) {
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
		owned := itemmodal.LibraryID(library.ID)
		if _, err := client.MediaSource.Delete().Where(sourcemodal.HasItemWith(owned)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if _, err := client.Item.Delete().Where(owned).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if _, err := client.Genre.Delete().Where(genremodal.Name(name)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the genre: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := items.New(client)
	movie, err := service.SaveScanned(ctx, items.Scanned{
		LibraryID: library.ID,
		Kind:      itemmodal.KindMovie,
		Name:      name,
		SortName:  name,
		Path:      "/" + name,
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}
	if err := service.SaveProbe(ctx, movie, items.Probe{Metadata: items.ContainerMetadata{Genres: []string{name}}}); err != nil {
		t.Fatalf("failed to save the probe: %v", err)
	}

	response, err := New(service).GetGenres(ctx, api.GetGenresRequestObject{
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
