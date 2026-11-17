package storetest

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/store/library"
	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
)

func Open(t testing.TB) *store.Store {
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
	t.Cleanup(func() { _ = connection.Stop() })

	return connection
}

func Client(t testing.TB) *store.Client {
	t.Helper()

	return Open(t).Client()
}

func Library(t testing.TB, client *store.Client, collectionType library.CollectionType) *store.LibraryModel {
	t.Helper()

	ctx := context.Background()
	created, err := client.Library.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetCollectionType(collectionType).
		SaveModel(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}
	t.Cleanup(func() {
		held := itemmodel.HasLibrariesWith(librarymembership.LibraryID(created.ID))
		if _, err := client.Item.Delete().Where(held).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the library's items: %v", err)
		}
		if err := client.Library.DeleteOneID(created.ID).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
	})

	return created
}
