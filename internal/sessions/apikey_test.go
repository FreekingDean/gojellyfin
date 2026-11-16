package sessions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
	policymodel "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

func (f *fixture) key(t *testing.T, administrator bool) (string, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	if err := f.client.UserPolicy.Create().SetUserID(f.user.ID).SetIsAdministrator(administrator).Exec(ctx); err != nil {
		t.Fatalf("failed to create the policy: %v", err)
	}

	token := uuid.NewString()
	key, err := f.client.ApiKey.Create().SetUserID(f.user.ID).SetAppName("Home Automation").SetAccessToken(token).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the api key: %v", err)
	}

	t.Cleanup(func() {
		if _, err := f.client.Session.Delete().Where(sessionmodel.APIKeyID(key.ID)).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the key's sessions: %v", err)
		}
		if _, err := f.client.Device.Delete().Where(devicemodel.ClientID(KeyClientID(key.ID))).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the key's device: %v", err)
		}
	})

	return token, key.ID
}

func (f *fixture) setPolicy(t *testing.T, administrator, disabled bool) {
	t.Helper()

	err := f.client.UserPolicy.Update().
		Where(policymodel.HasUserWith(usermodel.ID(f.user.ID))).
		SetIsAdministrator(administrator).
		SetIsDisabled(disabled).
		Exec(context.Background())
	if err != nil {
		t.Fatalf("failed to update the policy: %v", err)
	}
}

func TestService_ByToken_APIKey(t *testing.T) {
	t.Run("signs in as the administrator who made it, on a device named for the app", func(t *testing.T) {
		fixture := newFixture(t)
		token, keyID := fixture.key(t, true)

		session, err := fixture.service.ByToken(context.Background(), token)
		if err != nil {
			t.Fatalf("failed to authenticate with the key: %v", err)
		}
		if session.UserID == nil || *session.UserID != fixture.user.ID {
			t.Errorf("user = %v, want %v", session.UserID, fixture.user.ID)
		}
		if session.APIKeyID == nil || *session.APIKeyID != keyID {
			t.Errorf("api key = %v, want %v", session.APIKeyID, keyID)
		}
		if session.Device == nil || session.Device.ClientID != KeyClientID(keyID) || session.Device.Name != "Home Automation" {
			t.Errorf("device = %+v, want the key's own", session.Device)
		}

		again, err := fixture.service.ByToken(context.Background(), token)
		if err != nil {
			t.Fatalf("failed to authenticate with the key a second time: %v", err)
		}
		if again.ID != session.ID {
			t.Errorf("second session = %v, want the first one, %v, reused", again.ID, session.ID)
		}
	})

	t.Run("refuses a key whose maker is not an administrator", func(t *testing.T) {
		fixture := newFixture(t)
		token, _ := fixture.key(t, false)

		if _, err := fixture.service.ByToken(context.Background(), token); !store.IsNotFound(err) {
			t.Errorf("err = %v, want not found", err)
		}
	})

	t.Run("stops working when its maker is demoted or disabled", func(t *testing.T) {
		for name, policy := range map[string][2]bool{"demoted": {false, false}, "disabled": {true, true}} {
			t.Run(name, func(t *testing.T) {
				fixture := newFixture(t)
				token, _ := fixture.key(t, true)

				if _, err := fixture.service.ByToken(context.Background(), token); err != nil {
					t.Fatalf("failed to authenticate with the key: %v", err)
				}

				fixture.setPolicy(t, policy[0], policy[1])
				if _, err := fixture.service.ByToken(context.Background(), token); !store.IsNotFound(err) {
					t.Errorf("err = %v, want not found once the maker is %s", err, name)
				}
			})
		}
	})

	t.Run("stops working once revoked", func(t *testing.T) {
		fixture := newFixture(t)
		token, keyID := fixture.key(t, true)

		if _, err := fixture.service.ByToken(context.Background(), token); err != nil {
			t.Fatalf("failed to authenticate with the key: %v", err)
		}

		if err := fixture.client.ApiKey.UpdateOneID(keyID).SetRevokedAt(time.Now().Add(-time.Minute)).Exec(context.Background()); err != nil {
			t.Fatalf("failed to revoke the key: %v", err)
		}
		if _, err := fixture.service.ByToken(context.Background(), token); !store.IsNotFound(err) {
			t.Errorf("err = %v, want not found for a revoked key", err)
		}
	})

	t.Run("stops working once deleted", func(t *testing.T) {
		fixture := newFixture(t)
		token, keyID := fixture.key(t, true)

		if _, err := fixture.service.ByToken(context.Background(), token); err != nil {
			t.Fatalf("failed to authenticate with the key: %v", err)
		}

		if err := fixture.client.ApiKey.DeleteOneID(keyID).Exec(context.Background()); err != nil {
			t.Fatalf("failed to delete the key: %v", err)
		}
		if _, err := fixture.service.ByToken(context.Background(), token); !store.IsNotFound(err) {
			t.Errorf("err = %v, want not found for a deleted key", err)
		}
	})

	t.Run("survives its maker changing their password", func(t *testing.T) {
		fixture := newFixture(t)
		token, _ := fixture.key(t, true)

		if _, err := fixture.service.ByToken(context.Background(), token); err != nil {
			t.Fatalf("failed to authenticate with the key: %v", err)
		}
		if err := fixture.service.RevokeForUser(context.Background(), fixture.user.ID); err != nil {
			t.Fatalf("failed to revoke the user's sessions: %v", err)
		}
		if _, err := fixture.service.ByToken(context.Background(), token); err != nil {
			t.Errorf("err = %v, want the key to keep working", err)
		}
	})

	t.Run("an unknown token is still not found", func(t *testing.T) {
		fixture := newFixture(t)

		if _, err := fixture.service.ByToken(context.Background(), uuid.NewString()); !store.IsNotFound(err) {
			t.Errorf("err = %v, want not found", err)
		}
	})
}
