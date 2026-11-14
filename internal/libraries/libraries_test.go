package libraries

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"
)

func TestService_EnsureFixed(t *testing.T) {
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
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := client.Library.Delete().Where(librarymodel.IDIn(Movies.ID, Shows.ID)).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the fixed libraries: %v", err)
		}
	})

	service := New(client)
	for range 2 {
		if err := service.EnsureFixed(ctx); err != nil {
			t.Fatalf("EnsureFixed = %v", err)
		}
	}

	for _, fixed := range []Fixed{Movies, Shows} {
		found, err := client.Library.Query().
			Where(librarymodel.CollectionTypeEQ(fixed.CollectionType)).
			WithOptions().
			All(ctx)
		if err != nil {
			t.Fatalf("failed to read the libraries: %v", err)
		}
		if len(found) == 0 {
			t.Fatalf("no %s library after EnsureFixed", fixed.CollectionType)
		}
		for _, library := range found {
			if library.ID == fixed.ID && library.Edges.Options == nil {
				t.Errorf("%s has no options", fixed.Name)
			}
		}
	}
}
