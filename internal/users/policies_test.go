package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func TestSatisfies_AnAccountWithoutElevation(t *testing.T) {
	service := newService(t)
	ctx := context.Background()

	viewer, err := service.CreateUser(ctx, "viewer-"+uuid.NewString()[:8], "hash", false)
	if err != nil {
		t.Fatalf("failed to create a user: %v", err)
	}
	t.Cleanup(func() {
		if err := service.DeleteUser(context.Background(), viewer.ID); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
	})

	tests := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{"reads itself", []string{"DefaultAuthorization", "IgnoreParentalControl"}, true},
		{"reads the server", []string{"DefaultAuthorization", "FirstTimeSetupOrIgnoreParentalControl"}, true},
		{"administers", []string{"DefaultAuthorization", "RequiresElevation"}, false},
		{"names a scope nobody knows", []string{"DefaultAuthorization", "NotAScope"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.Satisfies(ctx, viewer.ID, tt.scopes)
			if err != nil {
				t.Fatalf("Satisfies failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("Satisfies(%v) = %v, want %v", tt.scopes, got, tt.want)
			}
		})
	}
}

func newService(t *testing.T) *users.Service {
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
	t.Cleanup(func() {
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return users.New(connection.Client())
}
