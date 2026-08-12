package user

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	configurationmodal "github.com/FreekingDean/gojellyfin/internal/store/userconfiguration"
	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func TestForgotPassword(t *testing.T) {
	server := New(nil, nil, nil)

	response, err := server.ForgotPassword(context.Background(), api.ForgotPasswordRequestObject{
		JSONBody: &api.ForgotPasswordJSONRequestBody{EnteredUsername: "Dean"},
	})
	if err != nil {
		t.Fatalf("ForgotPassword returned %v", err)
	}

	result, ok := response.(api.ForgotPassword200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.ForgotPassword200JSONResponse", response)
	}
	if *result.Action != api.ContactAdmin {
		t.Errorf("Action = %v, want %v", *result.Action, api.ContactAdmin)
	}
	if result.PinFile != nil || result.PinExpirationDate != nil {
		t.Errorf("PinFile = %v, PinExpirationDate = %v, want both unset", result.PinFile, result.PinExpirationDate)
	}
}

func TestForgotPasswordPin(t *testing.T) {
	server := New(nil, nil, nil)

	response, err := server.ForgotPasswordPin(context.Background(), api.ForgotPasswordPinRequestObject{
		JSONBody: &api.ForgotPasswordPinJSONRequestBody{Pin: "0000"},
	})
	if err != nil {
		t.Fatalf("ForgotPasswordPin returned %v", err)
	}

	result, ok := response.(api.ForgotPasswordPin200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.ForgotPasswordPin200JSONResponse", response)
	}
	if *result.Success {
		t.Error("Success = true, want false")
	}
	if len(*result.UsersReset) != 0 {
		t.Errorf("UsersReset = %v, want empty", *result.UsersReset)
	}
}

type fixture struct {
	server   *Server
	pending  *quickconnect.Service
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
	prefix := uuid.NewString() + "-"

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
		if _, err := client.UserPolicy.Delete().
			Where(policymodal.HasUserWith(usermodal.UsernameHasPrefix(prefix))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the user policies: %v", err)
		}
		if _, err := client.UserConfiguration.Delete().
			Where(configurationmodal.HasUserWith(usermodal.UsernameHasPrefix(prefix))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the user configurations: %v", err)
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

	pending := quickconnect.New()
	userService := users.New(client)
	sessionService := sessions.New(client)

	return &fixture{
		server:   New(userService, sessionService, pending),
		pending:  pending,
		sessions: sessionService,
		users:    userService,
		prefix:   prefix,
	}
}

func (f *fixture) deviceContext(device string) context.Context {
	return auth.ContextWithAuthorization(context.Background(), auth.Authorization{
		Client:   "Jellyfin Web",
		Device:   device,
		DeviceID: f.prefix + device,
		Version:  "10.10.0",
	})
}

func (f *fixture) account(t *testing.T, name string) uuid.UUID {
	t.Helper()

	account, err := f.users.CreateUser(context.Background(), f.prefix+name, "hash", false)
	if err != nil {
		t.Fatalf("failed to create the user %q: %v", name, err)
	}

	return account.ID
}

func (f *fixture) initiate(t *testing.T) quickconnect.Request {
	t.Helper()

	request, err := f.pending.Initiate(sessions.DeviceInfo{ID: f.prefix + "tv", Name: "tv", AppName: "Jellyfin Web"})
	if err != nil {
		t.Fatalf("failed to initiate quick connect: %v", err)
	}

	return request
}

func (f *fixture) redeem(t *testing.T, secret string) api.AuthenticateWithQuickConnectResponseObject {
	t.Helper()

	response, err := f.server.AuthenticateWithQuickConnect(f.deviceContext("tv"), api.AuthenticateWithQuickConnectRequestObject{
		JSONBody: &api.QuickConnectDto{Secret: secret},
	})
	if err != nil {
		t.Fatalf("failed to authenticate with quick connect: %v", err)
	}

	return response
}

func (f *fixture) refuseRedeem(t *testing.T, secret string) {
	t.Helper()

	if _, ok := f.redeem(t, secret).(api.AuthenticateWithQuickConnect400Response); !ok {
		t.Error("the secret was redeemable, want no token handed out")
	}
}

func TestAuthenticateWithQuickConnectNeedsASecret(t *testing.T) {
	fixture := newFixture(t)

	fixture.refuseRedeem(t, "")
	fixture.refuseRedeem(t, "not-a-secret")
}

func TestAuthenticateWithQuickConnectRefusesAnUnauthorizedRequest(t *testing.T) {
	fixture := newFixture(t)

	request := fixture.initiate(t)

	fixture.refuseRedeem(t, request.Secret)
}

func TestAuthenticateWithQuickConnectRefusesAnExpiredRequest(t *testing.T) {
	fixture := newFixture(t)
	userID := fixture.account(t, "dean")

	request := fixture.initiate(t)
	if err := fixture.pending.Authorize(request.Code, userID); err != nil {
		t.Fatalf("failed to authorize the request: %v", err)
	}
	fixture.pending.Expiry = time.Nanosecond

	fixture.refuseRedeem(t, request.Secret)
}

func TestAuthenticateWithQuickConnectIssuesASessionTokenOnce(t *testing.T) {
	fixture := newFixture(t)
	userID := fixture.account(t, "dean")

	request := fixture.initiate(t)
	if err := fixture.pending.Authorize(request.Code, userID); err != nil {
		t.Fatalf("failed to authorize the request: %v", err)
	}

	authenticated, ok := fixture.redeem(t, request.Secret).(api.AuthenticateWithQuickConnect200JSONResponse)
	if !ok {
		t.Fatal("the secret was not redeemable, want an access token")
	}
	if authenticated.AccessToken == nil || *authenticated.AccessToken == "" {
		t.Fatal("AccessToken is empty, want a usable token")
	}

	session, err := fixture.sessions.ByToken(context.Background(), *authenticated.AccessToken)
	if err != nil {
		t.Fatalf("failed to resolve the issued token: %v", err)
	}
	if session.Edges.User == nil || session.Edges.User.ID != userID {
		t.Errorf("session user = %v, want the authorizing user %v", session.Edges.User, userID)
	}

	fixture.refuseRedeem(t, request.Secret)
}
