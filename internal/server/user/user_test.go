package user

import (
	"context"
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
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
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

func TestServer_AuthenticateUserByName(t *testing.T) {
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
		server := New(users.New(client), sessions.New(client, activities), nil)

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
			if _, err := client.ActivityLogEntry.Delete().Where(entrymodel.HasUserWith(usermodel.ID(user.ID))).Exec(ctx); err != nil {
				t.Errorf("failed to delete the entries: %v", err)
			}
			if _, err := client.Session.Delete().Where(sessionmodel.HasUserWith(usermodel.ID(user.ID))).Exec(ctx); err != nil {
				t.Errorf("failed to delete the sessions: %v", err)
			}
			if _, err := client.Device.Delete().Where(devicemodel.ClientID(deviceID)).Exec(ctx); err != nil {
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
			if entry.User == nil || entry.User.ID != user.ID {
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
}

func TestServer_AuthenticateWithQuickConnect(t *testing.T) {
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
	sessionService := sessions.New(client, activity.New(client))
	pending := quickconnect.New(client)
	server := New(users.New(client), sessionService, pending)

	username := t.Name() + "-" + uuid.NewString()
	deviceID := uuid.NewString()
	user, err := client.User.Create().SetName(username).SetUsername(username).SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.ActivityLogEntry.Delete().Where(entrymodel.HasUserWith(usermodel.ID(user.ID))).Exec(ctx); err != nil {
			t.Errorf("failed to delete the entries: %v", err)
		}
		if _, err := client.Session.Delete().Where(sessionmodel.HasUserWith(usermodel.ID(user.ID))).Exec(ctx); err != nil {
			t.Errorf("failed to delete the sessions: %v", err)
		}
		if _, err := client.Device.Delete().Where(devicemodel.ClientID(deviceID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the device: %v", err)
		}
		if err := client.User.DeleteOne(user).Exec(ctx); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	ctx = auth.ContextWithAuthorization(ctx, auth.Authorization{Client: "Jellyfin Web", Device: "tv", DeviceID: deviceID})
	redeem := func(secret string) api.AuthenticateWithQuickConnectResponseObject {
		response, err := server.AuthenticateWithQuickConnect(ctx, api.AuthenticateWithQuickConnectRequestObject{
			JSONBody: &api.QuickConnectDto{Secret: secret},
		})
		if err != nil {
			t.Fatalf("failed to authenticate with quick connect: %v", err)
		}
		return response
	}

	for _, secret := range []string{"", "not-a-secret"} {
		if _, ok := redeem(secret).(api.AuthenticateWithQuickConnect400Response); !ok {
			t.Errorf("secret %q was redeemable, want 400", secret)
		}
	}

	request, err := pending.Initiate(ctx, auth.AuthorizationFrom(ctx).ClientDevice())
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	if err := pending.Authorize(ctx, request.Code, user.ID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	authenticated, ok := redeem(request.Secret).(api.AuthenticateWithQuickConnect200JSONResponse)
	if !ok || authenticated.AccessToken == nil {
		t.Fatal("the secret was not redeemable, want an access token")
	}
	session, err := sessionService.ByToken(ctx, *authenticated.AccessToken)
	if err != nil {
		t.Fatalf("failed to resolve the issued token: %v", err)
	}
	if session.User == nil || session.User.ID != user.ID {
		t.Errorf("session user = %v, want %v", session.User, user.ID)
	}

	if _, ok := redeem(request.Secret).(api.AuthenticateWithQuickConnect400Response); !ok {
		t.Error("the secret was redeemed twice, want 400")
	}
}
