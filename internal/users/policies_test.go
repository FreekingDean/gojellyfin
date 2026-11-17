package users_test

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

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

	connection := storetest.Open(t)
	t.Cleanup(func() {
	})

	return users.New(connection.Client())
}
