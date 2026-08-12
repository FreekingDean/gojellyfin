package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	auth     *Service
	sessions *sessions.Service
	users    *users.Service
	prefix   string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString() + "-"

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Session.Delete().
			Where(sessionmodal.HasDeviceWith(devicemodal.ClientIDHasPrefix(prefix))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the sessions: %v", err)
		}
		if _, err := client.Device.Delete().
			Where(devicemodal.ClientIDHasPrefix(prefix)).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the devices: %v", err)
		}
		if _, err := client.User.Delete().
			Where(usermodal.UsernameHasPrefix(prefix)).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	sessionService := sessions.New(client)

	return &fixture{
		auth:     New(sessionService),
		sessions: sessionService,
		users:    users.New(client),
		prefix:   prefix,
	}
}

func (f *fixture) signIn(t *testing.T, name string) (context.Context, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	user, err := f.users.CreateUser(ctx, f.prefix+name, "hash", false)
	if err != nil {
		t.Fatalf("failed to create the user %q: %v", name, err)
	}

	token := uuid.NewString()
	device := sessions.DeviceInfo{ID: f.prefix + name, Name: name, AppName: "Test", AppVersion: "1"}
	if _, err := f.sessions.Create(ctx, user.ID, token, device); err != nil {
		t.Fatalf("failed to create the session for %q: %v", name, err)
	}

	ctx, err = f.auth.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("failed to authenticate %q: %v", name, err)
	}

	return ctx, user.ID
}

func TestUserID(t *testing.T) {
	fixture := newFixture(t)

	ctx, id := fixture.signIn(t, "Dean")
	if got := UserID(ctx); got != id {
		t.Errorf("UserID = %v, want %v", got, id)
	}
	if got := UserID(context.Background()); got != uuid.Nil {
		t.Errorf("UserID = %v, want the nil uuid without a session", got)
	}
}

func TestAuthenticateRejectsAnUnknownToken(t *testing.T) {
	fixture := newFixture(t)

	if _, err := fixture.auth.Authenticate(context.Background(), ""); err != ErrUnauthorized {
		t.Errorf("Authenticate = %v, want %v for an empty token", err, ErrUnauthorized)
	}
	if _, err := fixture.auth.Authenticate(context.Background(), uuid.NewString()); err != ErrUnauthorized {
		t.Errorf("Authenticate = %v, want %v for an unknown token", err, ErrUnauthorized)
	}
}
