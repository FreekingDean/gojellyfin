package user

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	requestmodal "github.com/FreekingDean/gojellyfin/internal/store/quickconnectrequest"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	configurationmodal "github.com/FreekingDean/gojellyfin/internal/store/userconfiguration"
	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func TestServer_ForgotPassword(t *testing.T) {
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

func TestServer_ForgotPasswordPin(t *testing.T) {
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
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	pending := quickconnect.New(client)
	userService := users.New(client)
	sessionService := sessions.New(client, activity.New(client))

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

func (f *fixture) account(t *testing.T, name, password string) uuid.UUID {
	t.Helper()

	hash, err := auth.Hash(password)
	if err != nil {
		t.Fatalf("failed to hash the password: %v", err)
	}

	account, err := f.users.CreateUser(context.Background(), f.prefix+name, hash, false)
	if err != nil {
		t.Fatalf("failed to create the user %q: %v", name, err)
	}

	return account.ID
}

func (f *fixture) stored(t *testing.T, id uuid.UUID) *users.User {
	t.Helper()

	account, err := f.users.User(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to reload the user: %v", err)
	}

	return account
}

func (f *fixture) signIn(username, password string) (api.AuthenticateUserByNameResponseObject, error) {
	return f.server.AuthenticateUserByName(f.deviceContext("browser"), api.AuthenticateUserByNameRequestObject{
		JSONBody: &api.AuthenticateUserByName{Username: &username, Pw: &password},
	})
}

func (f *fixture) initiate(t *testing.T) *quickconnect.Request {
	t.Helper()

	request, err := f.pending.Initiate(context.Background(), sessions.DeviceInfo{ID: f.prefix + "tv", Name: "tv", AppName: "Jellyfin Web"})
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

func TestServer_AuthenticateUserByName(t *testing.T) {
	fixture := newFixture(t)

	t.Run("issues a session token the request can be resolved by", func(t *testing.T) {
		userID := fixture.account(t, "dean", "hunter2")

		response, err := fixture.signIn(fixture.prefix+"dean", "hunter2")
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		authenticated, ok := response.(api.AuthenticateUserByName200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want api.AuthenticateUserByName200JSONResponse", response)
		}
		if authenticated.AccessToken == nil || *authenticated.AccessToken == "" {
			t.Fatal("AccessToken is empty, want a usable token")
		}
		if authenticated.SessionInfo == nil || authenticated.SessionInfo.Id == nil {
			t.Fatal("SessionInfo is empty, want the created session")
		}
		if authenticated.User == nil || *authenticated.User.Id != userID {
			t.Errorf("User = %v, want %v", authenticated.User, userID)
		}

		session, err := fixture.sessions.ByToken(context.Background(), *authenticated.AccessToken)
		if err != nil {
			t.Fatalf("failed to resolve the issued token: %v", err)
		}
		if session.Edges.User == nil || session.Edges.User.ID != userID {
			t.Errorf("session user = %v, want %v", session.Edges.User, userID)
		}
		if session.ID.String() != *authenticated.SessionInfo.Id {
			t.Errorf("SessionInfo.Id = %q, want the created session %q", *authenticated.SessionInfo.Id, session.ID)
		}
		if fixture.stored(t, userID).LastLoginAt.IsZero() {
			t.Error("LastLoginAt is zero, want the login recorded")
		}
	})

	t.Run("records an authentication that leaks no secret", func(t *testing.T) {
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

		ctx := context.Background()
		client := connection.Client()
		activities := activity.New(client)
		server := New(users.New(client), sessions.New(client, activities), quickconnect.New(client))

		username := t.Name() + "-" + uuid.NewString()
		password := "hunter2"
		deviceID := uuid.NewString()

		hash, err := auth.Hash(password)
		if err != nil {
			t.Fatalf("failed to hash the password: %v", err)
		}
		user, err := client.User.Create().
			SetName(username).
			SetUsername(username).
			SetPasswordHash(hash).
			Save(ctx)
		if err != nil {
			t.Fatalf("failed to create the user: %v", err)
		}

		t.Cleanup(func() {
			if _, err := client.ActivityLogEntry.Delete().Where(entrymodal.HasUserWith(usermodal.ID(user.ID))).Exec(ctx); err != nil {
				t.Errorf("failed to delete the entries: %v", err)
			}
			if _, err := client.Session.Delete().Where(sessionmodal.HasUserWith(usermodal.ID(user.ID))).Exec(ctx); err != nil {
				t.Errorf("failed to delete the sessions: %v", err)
			}
			if _, err := client.Device.Delete().Where(devicemodal.ClientID(deviceID)).Exec(ctx); err != nil {
				t.Errorf("failed to delete the device: %v", err)
			}
			if err := client.User.DeleteOne(user).Exec(ctx); err != nil {
				t.Errorf("failed to delete the user: %v", err)
			}
			if err := connection.Stop(); err != nil {
				t.Errorf("failed to close the database: %v", err)
			}
		})

		ctx = auth.ContextWithAuthorization(ctx, auth.Authorization{
			Client:   "Jellyfin Web",
			Device:   "Firefox",
			DeviceID: deviceID,
			Version:  "10.10.0",
		})

		start := time.Now()
		response, err := server.AuthenticateUserByName(ctx, api.AuthenticateUserByNameRequestObject{
			JSONBody: &api.AuthenticateUserByNameJSONRequestBody{
				Username: apiutil.Ptr(username),
				Pw:       apiutil.Ptr(password),
			},
		})
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		result, ok := response.(api.AuthenticateUserByName200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want a 200", response)
		}
		token := apiutil.Deref(result.AccessToken)
		if token == "" {
			t.Fatal("access token is empty")
		}

		entries, _, err := activities.Entries(ctx, activity.Query{MinDate: &start, HasUserID: apiutil.Ptr(true)})
		if err != nil {
			t.Fatalf("failed to query the entries: %v", err)
		}

		found := 0
		for _, entry := range entries {
			if entry.Edges.User == nil || entry.Edges.User.ID != user.ID {
				continue
			}
			found++

			if entry.Kind != activity.KindAuthenticationSucceeded {
				t.Errorf("kind = %q, want %q", entry.Kind, activity.KindAuthenticationSucceeded)
			}
			if !strings.Contains(entry.Name, username) {
				t.Errorf("name = %q, want it to name %q", entry.Name, username)
			}
			if entry.ShortOverview != "Firefox" {
				t.Errorf("short overview = %q, want %q", entry.ShortOverview, "Firefox")
			}
			for field, value := range map[string]string{"name": entry.Name, "overview": entry.Overview, "short overview": entry.ShortOverview} {
				if strings.Contains(value, password) || strings.Contains(value, hash) || strings.Contains(value, token) {
					t.Errorf("%s leaks a secret: %q", field, value)
				}
			}
		}

		if found != 1 {
			t.Errorf("entries for the user = %d, want 1", found)
		}
	})

	t.Run("refuses credentials that do not match", func(t *testing.T) {
		fixture.account(t, "bad", "hunter2")

		if _, err := fixture.signIn(fixture.prefix+"bad", "wrong"); !errors.Is(err, auth.ErrUnauthorized) {
			t.Errorf("wrong password err = %v, want auth.ErrUnauthorized", err)
		}
		if _, err := fixture.signIn(fixture.prefix+"nobody", "hunter2"); !errors.Is(err, auth.ErrUnauthorized) {
			t.Errorf("unknown user err = %v, want auth.ErrUnauthorized", err)
		}
		if _, err := fixture.signIn(fixture.prefix+"bad", ""); !errors.Is(err, auth.ErrUnauthorized) {
			t.Errorf("empty password err = %v, want auth.ErrUnauthorized", err)
		}
	})
}

func TestServer_AuthenticateWithQuickConnect(t *testing.T) {
	fixture := newFixture(t)

	t.Run("refuses a secret nobody was issued", func(t *testing.T) {
		fixture.refuseRedeem(t, "")
		fixture.refuseRedeem(t, "not-a-secret")
	})

	t.Run("refuses a request nobody authorized", func(t *testing.T) {
		request := fixture.initiate(t)

		fixture.refuseRedeem(t, request.Secret)
	})

	t.Run("issues a session token for the authorizing user once", func(t *testing.T) {
		userID := fixture.account(t, "dean", "hunter2")

		request := fixture.initiate(t)
		if err := fixture.pending.Authorize(context.Background(), request.Code, userID); err != nil {
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
	})
}
