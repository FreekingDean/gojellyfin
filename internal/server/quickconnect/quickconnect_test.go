package quickconnect

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/server/user"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	configurationmodal "github.com/FreekingDean/gojellyfin/internal/store/userconfiguration"
	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	pending    *quickconnect.Service
	server     *Server
	userServer *user.Server
	sessions   *sessions.Service
	users      *users.Service
	auth       *auth.Service
	prefix     string
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
		pending:    pending,
		server:     New(pending, userService),
		userServer: user.New(userService, sessionService, pending),
		sessions:   sessionService,
		users:      userService,
		auth:       auth.New(sessionService),
		prefix:     prefix,
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

func (f *fixture) signedIn(t *testing.T, name string, administrator bool) (context.Context, uuid.UUID) {
	t.Helper()

	ctx := f.deviceContext(name + "-browser")
	account, err := f.users.CreateUser(ctx, f.prefix+name, "hash", administrator)
	if err != nil {
		t.Fatalf("failed to create the user %q: %v", name, err)
	}

	token, err := auth.NewToken()
	if err != nil {
		t.Fatalf("failed to mint a token: %v", err)
	}
	if _, err := f.sessions.Create(ctx, account.ID, token, auth.AuthorizationFrom(ctx).DeviceInfo()); err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	signedIn, err := f.auth.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("failed to authenticate the session: %v", err)
	}

	return signedIn, account.ID
}

func (f *fixture) initiate(t *testing.T, ctx context.Context) api.QuickConnectResult {
	t.Helper()

	response, err := f.server.InitiateQuickConnect(ctx, api.InitiateQuickConnectRequestObject{})
	if err != nil {
		t.Fatalf("failed to initiate quick connect: %v", err)
	}

	result, ok := response.(api.InitiateQuickConnect200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.InitiateQuickConnect200JSONResponse", response)
	}

	return api.QuickConnectResult(result)
}

func (f *fixture) authorize(t *testing.T, ctx context.Context, params api.AuthorizeQuickConnectParams) bool {
	t.Helper()

	response, err := f.server.AuthorizeQuickConnect(ctx, api.AuthorizeQuickConnectRequestObject{Params: params})
	if err != nil {
		t.Fatalf("failed to authorize quick connect: %v", err)
	}

	result, ok := response.(api.AuthorizeQuickConnect200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.AuthorizeQuickConnect200JSONResponse", response)
	}

	return bool(result)
}

func (f *fixture) redeem(t *testing.T, ctx context.Context, secret string) api.AuthenticateWithQuickConnectResponseObject {
	t.Helper()

	response, err := f.userServer.AuthenticateWithQuickConnect(ctx, api.AuthenticateWithQuickConnectRequestObject{
		JSONBody: &api.QuickConnectDto{Secret: secret},
	})
	if err != nil {
		t.Fatalf("failed to authenticate with quick connect: %v", err)
	}

	return response
}

func TestGetQuickConnectEnabledMatchesTheServerConfiguration(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.server.GetQuickConnectEnabled(context.Background(), api.GetQuickConnectEnabledRequestObject{})
	if err != nil {
		t.Fatalf("failed to get the quick connect state: %v", err)
	}

	result, ok := response.(api.GetQuickConnectEnabled200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetQuickConnectEnabled200JSONResponse", response)
	}

	available := configuration.DefaultServerConfiguration().QuickConnectAvailable
	if available == nil || *available != bool(result) {
		t.Errorf("GetQuickConnectEnabled = %v, want the same as QuickConnectAvailable %v", bool(result), available)
	}
}

func TestInitiateQuickConnectDescribesTheRequestingDevice(t *testing.T) {
	fixture := newFixture(t)

	result := fixture.initiate(t, fixture.deviceContext("tv"))

	if *result.Authenticated {
		t.Error("Authenticated = true, want a fresh request to be unauthorized")
	}
	if len(*result.Code) != 6 {
		t.Errorf("Code = %q, want six digits", *result.Code)
	}
	if len(*result.Secret) < 32 {
		t.Errorf("Secret = %q, want an unguessable value", *result.Secret)
	}
	if *result.Secret == *result.Code {
		t.Error("Secret equals Code, want the poll to need more than the displayed code")
	}
	if *result.DeviceId != fixture.prefix+"tv" || *result.DeviceName != "tv" || *result.AppName != "Jellyfin Web" {
		t.Errorf("device = %q/%q/%q, want the initiating device", *result.DeviceId, *result.DeviceName, *result.AppName)
	}
}

func TestInitiateQuickConnectIssuesDistinctCodes(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.deviceContext("tv")

	first := fixture.initiate(t, ctx)
	second := fixture.initiate(t, ctx)

	if *first.Code == *second.Code {
		t.Errorf("Code = %q twice, want pending codes to be distinct", *first.Code)
	}
	if *first.Secret == *second.Secret {
		t.Error("Secret repeated, want a fresh secret per request")
	}
}

func TestGetQuickConnectStateRejectsAnUnknownSecret(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.server.GetQuickConnectState(context.Background(), api.GetQuickConnectStateRequestObject{
		Params: api.GetQuickConnectStateParams{Secret: "not-a-secret"},
	})
	if err != nil {
		t.Fatalf("failed to get the quick connect state: %v", err)
	}

	if _, ok := response.(api.GetQuickConnectState404JSONResponse); !ok {
		t.Fatalf("response = %T, want api.GetQuickConnectState404JSONResponse", response)
	}
}

func TestAuthorizeQuickConnectRequiresAnAuthenticatedCaller(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.deviceContext("tv")

	result := fixture.initiate(t, ctx)

	_, err := fixture.server.AuthorizeQuickConnect(ctx, api.AuthorizeQuickConnectRequestObject{
		Params: api.AuthorizeQuickConnectParams{Code: *result.Code},
	})
	if !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("err = %v, want auth.ErrUnauthorized", err)
	}

	if _, ok := fixture.redeem(t, ctx, *result.Secret).(api.AuthenticateWithQuickConnect400Response); !ok {
		t.Error("the request was redeemable, want an unauthorized code to hand out nothing")
	}
}

func TestAuthorizeQuickConnectRejectsAnUnknownCode(t *testing.T) {
	fixture := newFixture(t)
	signedIn, _ := fixture.signedIn(t, "dean", false)

	if fixture.authorize(t, signedIn, api.AuthorizeQuickConnectParams{Code: "000000"}) {
		t.Error("AuthorizeQuickConnect = true, want false for a code that was never issued")
	}
}

func TestAuthorizeQuickConnectForAnotherUserNeedsAnAdministrator(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.deviceContext("tv")
	signedIn, _ := fixture.signedIn(t, "dean", false)
	_, otherID := fixture.signedIn(t, "other", false)

	result := fixture.initiate(t, ctx)

	response, err := fixture.server.AuthorizeQuickConnect(signedIn, api.AuthorizeQuickConnectRequestObject{
		Params: api.AuthorizeQuickConnectParams{Code: *result.Code, UserId: &otherID},
	})
	if err != nil {
		t.Fatalf("failed to authorize quick connect: %v", err)
	}
	if _, ok := response.(api.AuthorizeQuickConnect403JSONResponse); !ok {
		t.Fatalf("response = %T, want api.AuthorizeQuickConnect403JSONResponse", response)
	}

	if _, ok := fixture.redeem(t, ctx, *result.Secret).(api.AuthenticateWithQuickConnect400Response); !ok {
		t.Error("the request was redeemable, want a refused authorization to hand out nothing")
	}
}

func TestQuickConnectRejectsAnExpiredRequest(t *testing.T) {
	fixture := newFixture(t)
	fixture.pending.Expiry = time.Nanosecond

	ctx := fixture.deviceContext("tv")
	signedIn, _ := fixture.signedIn(t, "dean", false)
	result := fixture.initiate(t, ctx)

	state, err := fixture.server.GetQuickConnectState(ctx, api.GetQuickConnectStateRequestObject{
		Params: api.GetQuickConnectStateParams{Secret: *result.Secret},
	})
	if err != nil {
		t.Fatalf("failed to get the quick connect state: %v", err)
	}
	if _, ok := state.(api.GetQuickConnectState404JSONResponse); !ok {
		t.Fatalf("response = %T, want api.GetQuickConnectState404JSONResponse", state)
	}

	if fixture.authorize(t, signedIn, api.AuthorizeQuickConnectParams{Code: *result.Code}) {
		t.Error("AuthorizeQuickConnect = true, want an expired code to be refused")
	}

	if _, ok := fixture.redeem(t, ctx, *result.Secret).(api.AuthenticateWithQuickConnect400Response); !ok {
		t.Error("the request was redeemable, want an expired code to hand out nothing")
	}
}

func TestQuickConnectHandsOutASessionTokenOnce(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.deviceContext("tv")
	signedIn, userID := fixture.signedIn(t, "dean", false)

	result := fixture.initiate(t, ctx)

	if !fixture.authorize(t, signedIn, api.AuthorizeQuickConnectParams{Code: *result.Code}) {
		t.Fatal("AuthorizeQuickConnect = false, want the pending code to be authorized")
	}

	state, err := fixture.server.GetQuickConnectState(ctx, api.GetQuickConnectStateRequestObject{
		Params: api.GetQuickConnectStateParams{Secret: *result.Secret},
	})
	if err != nil {
		t.Fatalf("failed to get the quick connect state: %v", err)
	}
	polled, ok := state.(api.GetQuickConnectState200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetQuickConnectState200JSONResponse", state)
	}
	if !*polled.Authenticated {
		t.Error("Authenticated = false, want the poll to report the authorization")
	}

	authenticated, ok := fixture.redeem(t, ctx, *result.Secret).(api.AuthenticateWithQuickConnect200JSONResponse)
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

	if _, ok := fixture.redeem(t, ctx, *result.Secret).(api.AuthenticateWithQuickConnect400Response); !ok {
		t.Error("the secret was redeemable twice, want it consumed by the first exchange")
	}
}
