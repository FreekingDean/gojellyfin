package quickconnect

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	requestmodel "github.com/FreekingDean/gojellyfin/internal/store/quickconnectrequest"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	service *Service
	client  *store.Client
	prefix  string
}

func newFixture(t *testing.T) *fixture {
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

	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString() + "-"

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.QuickConnectRequest.Delete().
			Where(requestmodel.DeviceIDHasPrefix(prefix)).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the quick connect requests: %v", err)
		}
		if _, err := client.User.Delete().
			Where(usermodel.UsernameHasPrefix(prefix)).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{service: New(client), client: client, prefix: prefix}
}

func (f *fixture) account(t *testing.T, name string) uuid.UUID {
	t.Helper()

	account, err := users.New(f.client).CreateUser(context.Background(), f.prefix+name, "hash", false)
	if err != nil {
		t.Fatalf("failed to create the user %q: %v", name, err)
	}

	return account.ID
}

func (f *fixture) initiate(t *testing.T) *Request {
	t.Helper()

	request, err := f.service.Initiate(context.Background(), sessions.Device{
		ClientID: f.prefix + "tv", Name: "tv", AppName: "Jellyfin Web", AppVersion: "10.10.0",
	})
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	return request
}

func (f *fixture) authorized(t *testing.T, name string) (*Request, uuid.UUID) {
	t.Helper()

	request := f.initiate(t)
	userID := f.account(t, name)
	if err := f.service.Authorize(context.Background(), request.Code, userID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	return request, userID
}

func TestService_Initiate(t *testing.T) {
	fixture := newFixture(t)

	first := fixture.initiate(t)
	second := fixture.initiate(t)

	if first.DeviceName != "tv" || first.AppName != "Jellyfin Web" || first.AppVersion != "10.10.0" {
		t.Errorf("request = %+v, want it to describe the device that asked", first)
	}
	if len(first.Code) != 6 {
		t.Errorf("Code = %q, want six digits", first.Code)
	}
	if first.AuthorizedByID != nil {
		t.Error("AuthorizedByID is set, want a fresh request to be unauthorized")
	}
	if lifetime := first.ExpiresAt.Sub(first.CreatedAt); lifetime < 9*time.Minute || lifetime > 11*time.Minute {
		t.Errorf("lifetime = %v, want ten minutes", lifetime)
	}
	if first.Secret == second.Secret || first.Code == second.Code {
		t.Error("two requests share a secret or a code")
	}
}

func TestService_Flow(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	request, userID := fixture.authorized(t, "dean")

	polled, err := fixture.service.Pending(ctx, request.Secret)
	if err != nil {
		t.Fatalf("failed to poll: %v", err)
	}
	if polled.AuthorizedByID == nil || *polled.AuthorizedByID != userID {
		t.Errorf("AuthorizedByID = %v, want %v", polled.AuthorizedByID, userID)
	}

	if err := fixture.service.Authorize(ctx, request.Code, fixture.account(t, "other")); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want a taken code to be unauthorizable", err)
	}

	redeemed, err := fixture.service.Redeem(ctx, request.Secret)
	if err != nil {
		t.Fatalf("failed to redeem: %v", err)
	}
	if redeemed != userID {
		t.Errorf("redeemed user = %v, want %v", redeemed, userID)
	}

	if _, err := fixture.service.Redeem(ctx, request.Secret); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want a spent secret to be refused", err)
	}
}

func TestService_Refusals(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	t.Run("a request nobody authorized", func(t *testing.T) {
		request := fixture.initiate(t)

		if _, err := fixture.service.Redeem(ctx, request.Secret); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("the code in place of the secret", func(t *testing.T) {
		request, _ := fixture.authorized(t, "code")

		if _, err := fixture.service.Pending(ctx, request.Code); !errors.Is(err, ErrNotFound) {
			t.Errorf("Pending err = %v, want ErrNotFound", err)
		}
		if _, err := fixture.service.Redeem(ctx, request.Code); !errors.Is(err, ErrNotFound) {
			t.Errorf("Redeem err = %v, want ErrNotFound", err)
		}
	})

	t.Run("an expired request", func(t *testing.T) {
		request, _ := fixture.authorized(t, "expired")
		if err := fixture.client.QuickConnectRequest.UpdateOneID(request.ID).
			SetExpiresAt(time.Now().Add(-time.Second)).
			Exec(ctx); err != nil {
			t.Fatalf("failed to expire the request: %v", err)
		}

		if _, err := fixture.service.Pending(ctx, request.Secret); !errors.Is(err, ErrNotFound) {
			t.Errorf("Pending err = %v, want ErrNotFound", err)
		}
		if _, err := fixture.service.Redeem(ctx, request.Secret); !errors.Is(err, ErrNotFound) {
			t.Errorf("Redeem err = %v, want ErrNotFound", err)
		}
	})

	t.Run("an authorization whose user has gone", func(t *testing.T) {
		request, userID := fixture.authorized(t, "leaving")
		if err := fixture.client.User.DeleteOneID(userID).Exec(ctx); err != nil {
			t.Fatalf("failed to delete the user: %v", err)
		}

		if _, err := fixture.service.Redeem(ctx, request.Secret); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestService_RedeemRace(t *testing.T) {
	fixture := newFixture(t)
	request, userID := fixture.authorized(t, "racer")

	results := make([]error, 4)
	var group sync.WaitGroup
	for i := range results {
		group.Go(func() {
			redeemed, err := fixture.service.Redeem(context.Background(), request.Secret)
			if err == nil && redeemed != userID {
				t.Errorf("redeemed user = %v, want %v", redeemed, userID)
			}
			results[i] = err
		})
	}
	group.Wait()

	handed := 0
	for _, err := range results {
		if err == nil {
			handed++
		} else if !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound for a spent secret", err)
		}
	}
	if handed != 1 {
		t.Fatalf("the secret was redeemed %d times, want exactly once", handed)
	}
}
