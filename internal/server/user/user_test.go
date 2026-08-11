package user

import (
	"bytes"
	"context"
	"log"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/passwordresets"
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

const expiredTTL = -time.Minute

var (
	pinPattern      = regexp.MustCompile(`password reset pin for \S+: (\S+) `)
	passwordPattern = regexp.MustCompile(`password reset for \S+: new password (\S+)`)
)

type fixture struct {
	server *Server
	users  *users.Service
	client *store.Client
	prefix string
}

func newFixture(t *testing.T, ttl time.Duration) *fixture {
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
		mine := usermodal.UsernameHasPrefix(prefix)
		if _, err := client.Session.Delete().Where(sessionmodal.HasUserWith(mine)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the sessions: %v", err)
		}
		if _, err := client.Device.Delete().Where(devicemodal.ClientIDHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the devices: %v", err)
		}
		if _, err := client.UserConfiguration.Delete().Where(configurationmodal.HasUserWith(mine)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the user configurations: %v", err)
		}
		if _, err := client.UserPolicy.Delete().Where(policymodal.HasUserWith(mine)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the user policies: %v", err)
		}
		if _, err := client.User.Delete().Where(mine).Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := users.New(client)

	return &fixture{
		server: New(service, sessions.New(client), passwordresets.New(ttl)),
		users:  service,
		client: client,
		prefix: prefix,
	}
}

func (f *fixture) addUser(t *testing.T, name, password string, administrator bool) string {
	t.Helper()

	hash, err := auth.Hash(password)
	if err != nil {
		t.Fatalf("failed to hash the password: %v", err)
	}

	username := f.prefix + name
	if _, err := f.users.CreateUser(context.Background(), username, hash, administrator); err != nil {
		t.Fatalf("failed to create the user %q: %v", username, err)
	}

	return username
}

func (f *fixture) storedHash(t *testing.T, username string) string {
	t.Helper()

	user, err := f.users.UserByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("failed to read the user %q: %v", username, err)
	}

	return user.PasswordHash
}

func (f *fixture) forgot(t *testing.T, username string) (api.ForgotPassword200JSONResponse, string) {
	t.Helper()

	var response api.ForgotPasswordResponseObject
	output := captureLog(t, func() {
		var err error
		response, err = f.server.ForgotPassword(context.Background(), api.ForgotPasswordRequestObject{
			JSONBody: &api.ForgotPasswordJSONRequestBody{EnteredUsername: username},
		})
		if err != nil {
			t.Fatalf("failed to start the password reset: %v", err)
		}
	})

	result, ok := response.(api.ForgotPassword200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.ForgotPassword200JSONResponse", response)
	}

	pin := ""
	if matches := pinPattern.FindStringSubmatch(output); matches != nil {
		pin = matches[1]
	}

	return result, pin
}

func (f *fixture) redeem(t *testing.T, pin string) (api.ForgotPasswordPin200JSONResponse, string) {
	t.Helper()

	var response api.ForgotPasswordPinResponseObject
	output := captureLog(t, func() {
		var err error
		response, err = f.server.ForgotPasswordPin(context.Background(), api.ForgotPasswordPinRequestObject{
			JSONBody: &api.ForgotPasswordPinJSONRequestBody{Pin: pin},
		})
		if err != nil {
			t.Fatalf("failed to redeem the pin: %v", err)
		}
	})

	result, ok := response.(api.ForgotPasswordPin200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.ForgotPasswordPin200JSONResponse", response)
	}

	password := ""
	if matches := passwordPattern.FindStringSubmatch(output); matches != nil {
		password = matches[1]
	}

	return result, password
}

func (f *fixture) authenticates(t *testing.T, username, password string) bool {
	t.Helper()

	ctx := auth.ContextWithAuthorization(context.Background(), auth.Authorization{
		Client:   "Tests",
		Device:   "Tests",
		DeviceID: f.prefix + "device",
		Version:  "1",
	})

	response, err := f.server.AuthenticateUserByName(ctx, api.AuthenticateUserByNameRequestObject{
		JSONBody: &api.AuthenticateUserByNameJSONRequestBody{
			Username: &username,
			Pw:       &password,
		},
	})
	if err != nil {
		return false
	}

	if _, ok := response.(api.AuthenticateUserByName200JSONResponse); !ok {
		t.Fatalf("response = %T, want api.AuthenticateUserByName200JSONResponse", response)
	}

	return true
}

func captureLog(t *testing.T, run func()) string {
	t.Helper()

	buffer := &bytes.Buffer{}
	flags := log.Flags()
	log.SetOutput(buffer)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(flags)
	}()

	run()

	return buffer.String()
}

func TestForgotPassword(t *testing.T) {
	fixture := newFixture(t, passwordresets.DefaultTTL)
	username := fixture.addUser(t, "Dean", "hunter2", false)

	t.Run("answers an unknown username the same way as a known one", func(t *testing.T) {
		known, pin := fixture.forgot(t, username)
		unknown, missing := fixture.forgot(t, fixture.prefix+"nobody")

		if pin == "" {
			t.Error("no pin was issued for the known user")
		}
		if missing != "" {
			t.Errorf("a pin was issued for an unknown user: %q", missing)
		}
		if *known.Action != api.PinCode || *unknown.Action != api.PinCode {
			t.Errorf("actions = %v, %v, want both %v", *known.Action, *unknown.Action, api.PinCode)
		}
		if *known.PinFile != *unknown.PinFile {
			t.Errorf("pin files = %q, %q, want them to match", *known.PinFile, *unknown.PinFile)
		}
		if !unknown.PinExpirationDate.After(time.Now()) {
			t.Errorf("PinExpirationDate = %v, want a future time", *unknown.PinExpirationDate)
		}
	})

	t.Run("issues no pin for an administrator", func(t *testing.T) {
		administrator := fixture.addUser(t, "Admin", "hunter2", true)

		if _, pin := fixture.forgot(t, administrator); pin != "" {
			t.Errorf("a pin was issued for an administrator: %q", pin)
		}
	})

	t.Run("replaces an earlier pin", func(t *testing.T) {
		_, first := fixture.forgot(t, username)
		_, second := fixture.forgot(t, username)

		if result, _ := fixture.redeem(t, first); *result.Success {
			t.Error("the replaced pin was still redeemable")
		}
		if result, _ := fixture.redeem(t, second); !*result.Success {
			t.Error("the newest pin was refused")
		}
	})
}

func TestForgotPasswordPin(t *testing.T) {
	t.Run("resets the password", func(t *testing.T) {
		fixture := newFixture(t, passwordresets.DefaultTTL)
		username := fixture.addUser(t, "Dean", "hunter2", false)

		_, pin := fixture.forgot(t, username)
		result, password := fixture.redeem(t, pin)

		if !*result.Success {
			t.Fatal("the pin was refused")
		}
		if len(*result.UsersReset) != 1 || (*result.UsersReset)[0] != username {
			t.Errorf("UsersReset = %v, want [%q]", *result.UsersReset, username)
		}
		if password == "" {
			t.Fatal("no replacement password was issued")
		}
		if fixture.authenticates(t, username, "hunter2") {
			t.Error("the old password still authenticates")
		}
		if !fixture.authenticates(t, username, password) {
			t.Error("the replacement password does not authenticate")
		}
	})

	t.Run("refuses a used pin", func(t *testing.T) {
		fixture := newFixture(t, passwordresets.DefaultTTL)
		username := fixture.addUser(t, "Dean", "hunter2", false)

		_, pin := fixture.forgot(t, username)
		fixture.redeem(t, pin)
		before := fixture.storedHash(t, username)

		result, _ := fixture.redeem(t, pin)
		if *result.Success {
			t.Error("the pin was redeemed twice")
		}
		if after := fixture.storedHash(t, username); after != before {
			t.Error("the password changed on the second redemption")
		}
	})

	t.Run("refuses an expired pin", func(t *testing.T) {
		fixture := newFixture(t, expiredTTL)
		username := fixture.addUser(t, "Dean", "hunter2", false)

		_, pin := fixture.forgot(t, username)
		if pin == "" {
			t.Fatal("no pin was issued")
		}

		result, _ := fixture.redeem(t, pin)
		if *result.Success {
			t.Error("an expired pin was redeemed")
		}
		if !fixture.authenticates(t, username, "hunter2") {
			t.Error("the password changed on an expired pin")
		}
	})

	t.Run("refuses a wrong pin without saying whether one is pending", func(t *testing.T) {
		fixture := newFixture(t, passwordresets.DefaultTTL)
		username := fixture.addUser(t, "Dean", "hunter2", false)

		idle, _ := fixture.redeem(t, "not-a-pin")

		fixture.forgot(t, username)
		pending, _ := fixture.redeem(t, "not-a-pin")

		if *idle.Success || *pending.Success {
			t.Error("a wrong pin was accepted")
		}
		if len(*idle.UsersReset) != len(*pending.UsersReset) {
			t.Errorf("UsersReset = %v, %v, want them to match", *idle.UsersReset, *pending.UsersReset)
		}
		if !fixture.authenticates(t, username, "hunter2") {
			t.Error("a wrong pin changed the password")
		}
	})
}
