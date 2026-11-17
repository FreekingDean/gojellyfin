package storetest

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
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

func Scratch(t testing.TB) *store.Store {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	server, err := sql.Open("pgx", config.DatabaseURL)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	name := "gojellyfin_scratch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := server.Exec(fmt.Sprintf("CREATE DATABASE %s", name)); err != nil {
		t.Fatalf("failed to create a scratch database, set DATABASE_URL: %v", err)
	}
	t.Cleanup(func() {
		if _, err := server.Exec(fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", name)); err != nil {
			t.Errorf("failed to drop %s: %v", name, err)
		}
	})

	scratch, err := url.Parse(config.DatabaseURL)
	if err != nil {
		t.Fatalf("failed to parse DATABASE_URL: %v", err)
	}
	scratch.Path = "/" + name

	connection, err := store.NewStore(env.Config{DatabaseURL: scratch.String()})
	if err != nil {
		t.Fatalf("failed to open %s: %v", name, err)
	}
	t.Cleanup(func() { _ = connection.Stop() })

	if err := connection.Client().Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed to create the schema: %v", err)
	}

	return connection
}
