package quickconnect

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

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

	connection := storetest.Open(t)

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

	return f.initiateFrom(t, "192.0.2.1:40000")
}

func (f *fixture) initiateFrom(t *testing.T, address string) *Request {
	t.Helper()

	request, err := f.service.Initiate(context.Background(), sessions.Device{
		ClientID: f.prefix + "tv", Name: "tv", AppName: "Jellyfin Web", AppVersion: "10.10.0",
	}, address)
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
	if first.RemoteAddress != "192.0.2.1:40000" {
		t.Errorf("RemoteAddress = %q, want the address that asked", first.RemoteAddress)
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

func TestService_Waiting(t *testing.T) {
	fixture := newFixture(t)

	older := fixture.initiate(t)
	approved, _ := fixture.authorized(t, "approver")
	newer := fixture.initiate(t)

	waiting, err := fixture.service.Waiting(context.Background())
	if err != nil {
		t.Fatalf("Waiting returned %v", err)
	}

	var codes []string
	for _, request := range waiting {
		if request.DeviceID == older.DeviceID {
			codes = append(codes, request.Code)
		}
		if request.Code == approved.Code {
			t.Errorf("Waiting returned %s, which is already approved", approved.Code)
		}
	}
	if len(codes) != 2 || codes[0] != newer.Code || codes[1] != older.Code {
		t.Errorf("Waiting = %v, want the newest first: %s then %s", codes, newer.Code, older.Code)
	}
}

func TestService_Nearby(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	home := fixture.initiateFrom(t, "198.51.100.7:40000")
	fixture.initiateFrom(t, "198.51.100.8:40000")
	lan := fixture.initiateFrom(t, "10.1.20.31:40000")
	taken, _ := fixture.authorized(t, "dean")
	if _, err := fixture.client.QuickConnectRequest.UpdateOneID(taken.ID).SetRemoteAddress("198.51.100.7:1").Save(ctx); err != nil {
		t.Fatalf("failed to move the authorized request: %v", err)
	}

	tests := []struct {
		name    string
		address string
		want    []string
	}{
		{name: "finds the sign-in from the same public address", address: "198.51.100.7:51000", want: []string{home.Code}},
		{name: "finds the sign-in from the same private network", address: "10.1.20.44:51000", want: []string{lan.Code}},
		{name: "finds nothing from another private network", address: "10.1.21.44:51000"},
		{name: "finds nothing from an unreadable address", address: "phone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			near, err := fixture.service.Nearby(ctx, tt.address)
			if err != nil {
				t.Fatalf("failed to look nearby: %v", err)
			}

			codes := []string{}
			for _, request := range near {
				if strings.HasPrefix(request.DeviceID, fixture.prefix) {
					codes = append(codes, request.Code)
				}
			}
			if tt.want == nil {
				tt.want = []string{}
			}
			if !slices.Equal(codes, tt.want) {
				t.Errorf("codes = %v, want %v", codes, tt.want)
			}
		})
	}
}

func TestSameNetwork(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"198.51.100.7:1", "198.51.100.7:2", true},
		{"198.51.100.7:1", "198.51.100.8:2", false},
		{"10.1.20.31:1", "10.1.20.44:2", true},
		{"10.1.20.31:1", "10.1.21.31:2", false},
		{"127.0.0.1:1", "127.0.0.1:2", true},
		{"[2001:db8:1:2::10]:1", "[2001:db8:1:2::99]:2", true},
		{"[2001:db8:1:2::10]:1", "[2001:db8:1:3::10]:2", false},
		{"[::ffff:198.51.100.7]:1", "198.51.100.7:2", true},
		{"[::1]:1", "[::1]:2", true},
		{"198.51.100.7:1", "[2001:db8:1:2::10]:2", false},
		{"", "198.51.100.7:2", false},
		{"10.1.20.31:1", "198.51.100.7:2", false},
	}
	for _, tt := range tests {
		if got := SameNetwork(tt.a, tt.b); got != tt.want {
			t.Errorf("SameNetwork(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
