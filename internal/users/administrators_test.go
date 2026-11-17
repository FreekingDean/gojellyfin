package users

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/storetest"
)

func scratchService(t *testing.T) *Service {
	t.Helper()

	return New(storetest.Scratch(t).Client())
}

func TestService_KeepsAnAdministrator(t *testing.T) {
	ctx := context.Background()
	demote := func(policy *Policy) { policy.IsAdministrator = false }

	t.Run("refuses demoting the last administrator", func(t *testing.T) {
		service := scratchService(t)
		admin := create(t, service, true)

		if err := service.SavePolicy(ctx, admin.ID, demote); !errors.Is(err, ErrLastAdministrator) {
			t.Fatalf("SavePolicy = %v, want ErrLastAdministrator", err)
		}
		if administrator, _ := service.IsAdministrator(ctx, admin.ID); !administrator {
			t.Error("the refused demotion was written anyway")
		}
	})

	t.Run("refuses deleting the last administrator", func(t *testing.T) {
		service := scratchService(t)
		admin := create(t, service, true)
		create(t, service, false)

		if err := service.DeleteUser(ctx, admin.ID); !errors.Is(err, ErrLastAdministrator) {
			t.Fatalf("DeleteUser = %v, want ErrLastAdministrator", err)
		}
		if _, err := service.User(ctx, admin.ID); err != nil {
			t.Errorf("the refused delete removed the administrator: %v", err)
		}
	})

	t.Run("lets one of two administrators go", func(t *testing.T) {
		service := scratchService(t)
		first := create(t, service, true)
		second := create(t, service, true)

		if err := service.SavePolicy(ctx, first.ID, demote); err != nil {
			t.Fatalf("SavePolicy = %v, want the demotion allowed", err)
		}
		if err := service.DeleteUser(ctx, second.ID); !errors.Is(err, ErrLastAdministrator) {
			t.Errorf("DeleteUser = %v, want ErrLastAdministrator once only one is left", err)
		}
	})

	t.Run("leaves a server with no administrator to its other writes", func(t *testing.T) {
		service := scratchService(t)
		member := create(t, service, false)

		if err := service.SavePolicy(ctx, member.ID, func(policy *Policy) { policy.IsHidden = true }); err != nil {
			t.Errorf("SavePolicy = %v, want a member's change allowed", err)
		}
		if err := service.DeleteUser(ctx, member.ID); err != nil {
			t.Errorf("DeleteUser = %v, want a member deleted", err)
		}
	})
}

func create(t *testing.T, service *Service, administrator bool) *User {
	t.Helper()

	user, err := service.CreateUser(context.Background(), "user-"+uuid.NewString(), "hash", administrator)
	if err != nil {
		t.Fatalf("failed to create a user: %v", err)
	}

	return user
}
