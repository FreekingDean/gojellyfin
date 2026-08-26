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
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func TestServer_ForgotPassword(t *testing.T) {
	server := New(nil, nil)

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
	server := New(nil, nil)

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

func TestAuthenticateUserByNameRecordsActivity(t *testing.T) {
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
	server := New(users.New(client), sessions.New(client, activities))

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
}
