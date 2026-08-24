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
	requestmodal "github.com/FreekingDean/gojellyfin/internal/store/quickconnectrequest"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	service *Service
	client  *store.Client
	config  env.Config
	prefix  string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	prefix := t.Name() + "-" + uuid.NewString() + "-"
	client := connect(t, config)

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.QuickConnectRequest.Delete().
			Where(requestmodal.DeviceIDHasPrefix(prefix)).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the quick connect requests: %v", err)
		}
		if _, err := client.User.Delete().
			Where(usermodal.UsernameHasPrefix(prefix)).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
	})

	return &fixture{service: New(client), client: client, config: config, prefix: prefix}
}

func connect(t *testing.T, config env.Config) *store.Client {
	t.Helper()

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

	return connection.Client()
}

func (f *fixture) replica(t *testing.T) *Service {
	t.Helper()

	return New(connect(t, f.config))
}

func (f *fixture) expiring(expiry time.Duration) *Service {
	return &Service{store: f.client, expiry: expiry, maxPending: defaultMaxPending}
}

func (f *fixture) bounded(maxPending int) *Service {
	return &Service{store: f.client, expiry: defaultExpiry, maxPending: maxPending}
}

func (f *fixture) device(name string) sessions.DeviceInfo {
	return sessions.DeviceInfo{ID: f.prefix + name, Name: name, AppName: "Jellyfin Web", AppVersion: "10.10.0"}
}

func (f *fixture) account(t *testing.T, name string) uuid.UUID {
	t.Helper()

	account, err := users.New(f.client).CreateUser(context.Background(), f.prefix+name, "hash", false)
	if err != nil {
		t.Fatalf("failed to create the user %q: %v", name, err)
	}

	return account.ID
}

func (f *fixture) initiate(t *testing.T, service *Service) *Request {
	t.Helper()

	request, err := service.Initiate(context.Background(), f.device("tv"))
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	return request
}

func TestInitiateDescribesAFreshRequest(t *testing.T) {
	fixture := newFixture(t)

	request := fixture.initiate(t, fixture.service)

	if request.AuthorizedByID != uuid.Nil {
		t.Error("AuthorizedByID is set, want a fresh request to be unauthorized")
	}
	if len(request.Code) != 6 {
		t.Errorf("Code = %q, want six digits", request.Code)
	}
	if len(request.Secret) < 32 {
		t.Errorf("Secret = %q, want an unguessable value", request.Secret)
	}
	if request.DeviceID != fixture.prefix+"tv" || request.AppName != "Jellyfin Web" {
		t.Errorf("device = %q/%q, want the initiating device", request.DeviceID, request.AppName)
	}
	if request.ExpiresAt.Sub(request.CreatedAt) < defaultExpiry-time.Second {
		t.Errorf("ExpiresAt = %v, want %v after CreatedAt %v", request.ExpiresAt, defaultExpiry, request.CreatedAt)
	}
}

func TestInitiateIssuesDistinctSecretsAndCodes(t *testing.T) {
	fixture := newFixture(t)

	first := fixture.initiate(t, fixture.service)
	second := fixture.initiate(t, fixture.service)

	if first.Secret == second.Secret {
		t.Error("Secret repeated, want a fresh secret per request")
	}
	if first.Code == second.Code {
		t.Errorf("Code = %q twice, want pending codes to be distinct", first.Code)
	}
}

func TestInitiateBoundsThePendingRequests(t *testing.T) {
	fixture := newFixture(t)
	service := fixture.bounded(4)

	var err error
	for range 5 {
		if _, err = service.Initiate(context.Background(), fixture.device("tv")); err != nil {
			break
		}
	}

	if !errors.Is(err, ErrTooManyPending) {
		t.Fatalf("err = %v, want ErrTooManyPending", err)
	}
}

func TestPendingIsVisibleToAnotherReplica(t *testing.T) {
	fixture := newFixture(t)
	initiating := fixture.service
	authorizing := fixture.replica(t)
	redeeming := fixture.replica(t)
	userID := fixture.account(t, "dean")
	ctx := context.Background()

	request := fixture.initiate(t, initiating)

	polled, err := authorizing.Pending(ctx, request.Secret)
	if err != nil {
		t.Fatalf("a replica that did not initiate could not poll: %v", err)
	}
	if polled.Code != request.Code {
		t.Errorf("Code = %q, want the initiated code %q", polled.Code, request.Code)
	}

	if err := authorizing.Authorize(ctx, request.Code, userID); err != nil {
		t.Fatalf("a replica that did not initiate could not authorize: %v", err)
	}

	polled, err = initiating.Pending(ctx, request.Secret)
	if err != nil {
		t.Fatalf("failed to poll: %v", err)
	}
	if polled.AuthorizedByID != userID {
		t.Errorf("AuthorizedByID = %v, want the authorizing user %v", polled.AuthorizedByID, userID)
	}

	redeemed, err := redeeming.Redeem(ctx, request.Secret)
	if err != nil {
		t.Fatalf("a replica that did not initiate could not redeem: %v", err)
	}
	if redeemed != userID {
		t.Errorf("redeemed user = %v, want %v", redeemed, userID)
	}
}

func TestTheCodeIsNotThePollCredential(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	request := fixture.initiate(t, fixture.service)
	if err := fixture.service.Authorize(ctx, request.Code, fixture.account(t, "dean")); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	if _, err := fixture.service.Pending(ctx, request.Code); !errors.Is(err, ErrUnknownSecret) {
		t.Errorf("Pending(code) err = %v, want ErrUnknownSecret", err)
	}
	if _, err := fixture.service.Redeem(ctx, request.Code); !errors.Is(err, ErrUnknownSecret) {
		t.Errorf("Redeem(code) err = %v, want ErrUnknownSecret", err)
	}
}

func TestAuthorizeCannotDistinguishUnknownExpiredAndUsedCodes(t *testing.T) {
	fixture := newFixture(t)
	userID := fixture.account(t, "dean")
	ctx := context.Background()

	expiring := fixture.initiate(t, fixture.expiring(time.Nanosecond))

	redeemed := fixture.initiate(t, fixture.service)
	if err := fixture.service.Authorize(ctx, redeemed.Code, userID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}
	if _, err := fixture.service.Redeem(ctx, redeemed.Secret); err != nil {
		t.Fatalf("failed to redeem: %v", err)
	}

	if err := fixture.service.Authorize(ctx, "000000", userID); !errors.Is(err, ErrUnknownCode) {
		t.Errorf("unknown code err = %v, want ErrUnknownCode", err)
	}
	if err := fixture.service.Authorize(ctx, expiring.Code, userID); !errors.Is(err, ErrUnknownCode) {
		t.Errorf("expired code err = %v, want ErrUnknownCode", err)
	}
	if err := fixture.service.Authorize(ctx, redeemed.Code, userID); !errors.Is(err, ErrUnknownCode) {
		t.Errorf("redeemed code err = %v, want ErrUnknownCode", err)
	}
}

func TestAuthorizeRefusesToRebindAPendingCode(t *testing.T) {
	fixture := newFixture(t)
	userID := fixture.account(t, "dean")
	other := fixture.account(t, "other")
	ctx := context.Background()

	request := fixture.initiate(t, fixture.service)
	if err := fixture.service.Authorize(ctx, request.Code, userID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	if err := fixture.replica(t).Authorize(ctx, request.Code, other); !errors.Is(err, ErrAlreadyAuthorized) {
		t.Fatalf("err = %v, want ErrAlreadyAuthorized", err)
	}

	authorized, err := fixture.service.Redeem(ctx, request.Secret)
	if err != nil {
		t.Fatalf("failed to redeem: %v", err)
	}
	if authorized != userID {
		t.Errorf("authorized user = %v, want the first authorizing user %v", authorized, userID)
	}
}

func TestRedeemRefusesAnUnauthorizedRequest(t *testing.T) {
	fixture := newFixture(t)

	request := fixture.initiate(t, fixture.service)

	if _, err := fixture.service.Redeem(context.Background(), request.Secret); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want ErrNotAuthorized", err)
	}
}

func TestRedeemRefusesAnExpiredRequest(t *testing.T) {
	fixture := newFixture(t)
	userID := fixture.account(t, "dean")
	ctx := context.Background()

	request := fixture.initiate(t, fixture.expiring(time.Nanosecond))

	if err := fixture.service.Authorize(ctx, request.Code, userID); !errors.Is(err, ErrUnknownCode) {
		t.Fatalf("err = %v, want an expired code to be unauthorizable", err)
	}
	if _, err := fixture.service.Redeem(ctx, request.Secret); !errors.Is(err, ErrUnknownSecret) {
		t.Fatalf("err = %v, want ErrUnknownSecret", err)
	}
}

func TestRedeemIsSingleUseAcrossReplicas(t *testing.T) {
	fixture := newFixture(t)
	userID := fixture.account(t, "dean")
	ctx := context.Background()

	request := fixture.initiate(t, fixture.service)
	if err := fixture.service.Authorize(ctx, request.Code, userID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	replicas := []*Service{fixture.service, fixture.replica(t), fixture.replica(t), fixture.replica(t)}
	redeemed := make([]uuid.UUID, len(replicas))
	failures := make([]error, len(replicas))

	var group sync.WaitGroup
	for i, replica := range replicas {
		group.Add(1)
		go func() {
			defer group.Done()

			redeemed[i], failures[i] = replica.Redeem(ctx, request.Secret)
		}()
	}
	group.Wait()

	handed := 0
	for i, err := range failures {
		if err == nil {
			handed++

			if redeemed[i] != userID {
				t.Errorf("redeemed user = %v, want %v", redeemed[i], userID)
			}

			continue
		}
		if !errors.Is(err, ErrUnknownSecret) {
			t.Errorf("err = %v, want ErrUnknownSecret for a spent secret", err)
		}
	}
	if handed != 1 {
		t.Fatalf("the secret was redeemed %d times, want exactly once", handed)
	}
}

func TestDeletingTheUserDropsTheAuthorization(t *testing.T) {
	fixture := newFixture(t)
	userID := fixture.account(t, "dean")
	ctx := context.Background()

	request := fixture.initiate(t, fixture.service)
	if err := fixture.service.Authorize(ctx, request.Code, userID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	if err := fixture.client.User.DeleteOneID(userID).Exec(ctx); err != nil {
		t.Fatalf("failed to delete the user: %v", err)
	}

	if _, err := fixture.service.Redeem(ctx, request.Secret); !errors.Is(err, ErrUnknownSecret) {
		t.Fatalf("err = %v, want the authorization to go with the user", err)
	}
}
