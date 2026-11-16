package config

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/configuration"
)

func TestService_read(t *testing.T) {
	environment, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}
	connection, err := store.NewStore(environment)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	client := connection.Client()
	service := New(client)
	key := "test-" + uuid.NewString()
	t.Cleanup(func() {
		if _, err := client.Configuration.Delete().Where(configuration.Key(key)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the row: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	unset := DefaultServer()
	if err := service.read(ctx, key, &unset); err != nil || unset != DefaultServer() {
		t.Errorf("no row read as %+v, %v, want the defaults", unset, err)
	}

	stored := `{"ServerName":"Example","CachePath":"/cache","IsStartupWizardCompleted":null,"QuickConnectAvailable":true}`
	if err := service.SetConfiguration(ctx, key, []byte(stored)); err != nil {
		t.Fatalf("failed to store the row: %v", err)
	}

	read := DefaultServer()
	if err := service.read(ctx, key, &read); err != nil {
		t.Fatalf("read: %v", err)
	}
	want := Server{ServerName: "Example", CachePath: "/cache", IsStartupWizardCompleted: true}
	if read != want {
		t.Errorf("read %+v, want %+v", read, want)
	}
}
