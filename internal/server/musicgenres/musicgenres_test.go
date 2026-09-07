package musicgenres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	genremodal "github.com/FreekingDean/gojellyfin/internal/store/genre"
)

func TestServer_GetMusicGenre(t *testing.T) {
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

	t.Cleanup(func() {
		if _, err := client.Genre.Delete().Where(genremodal.Name(name)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the genre: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	if err := client.Genre.Create().SetName(name).Exec(ctx); err != nil {
		t.Fatalf("failed to create the genre: %v", err)
	}

	server := New(items.New(client))

	t.Run("answers a genre that exists", func(t *testing.T) {
		response, err := server.GetMusicGenre(ctx, api.GetMusicGenreRequestObject{GenreName: name})
		if err != nil {
			t.Fatalf("failed to get the music genre: %v", err)
		}

		result, ok := response.(api.GetMusicGenre200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want api.GetMusicGenre200JSONResponse", response)
		}
		if result.Name == nil || *result.Name != name {
			t.Errorf("name = %v, want %s", result.Name, name)
		}
		if result.Type == nil || *result.Type != api.BaseItemKindMusicGenre {
			t.Errorf("type = %v, want %s", result.Type, api.BaseItemKindMusicGenre)
		}
	})

	t.Run("refuses a name that matches nothing", func(t *testing.T) {
		response, err := server.GetMusicGenre(ctx, api.GetMusicGenreRequestObject{GenreName: name + "-absent"})
		if err != nil {
			t.Fatalf("a missing genre returned an error rather than a response: %v", err)
		}
		if _, ok := response.(api.GetMusicGenre403Response); !ok {
			t.Errorf("response = %T, want api.GetMusicGenre403Response", response)
		}
	})
}
