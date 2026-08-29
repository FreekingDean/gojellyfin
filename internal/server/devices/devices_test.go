package devices

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type fixture struct {
	server *Server
	client *store.Client
	prefix string
}

type deviceSeed struct {
	name         string
	customName   string
	appName      string
	appVersion   string
	iconURL      string
	lastActivity time.Time
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

	return &fixture{server: New(sessions.New(client, activity.New(client))), client: client, prefix: prefix}
}

func (f *fixture) addDevice(t *testing.T, device deviceSeed) string {
	t.Helper()

	clientID := f.prefix + device.name
	_, err := f.client.Device.Create().
		SetClientID(clientID).
		SetName(device.name).
		SetCustomName(device.customName).
		SetAppName(device.appName).
		SetAppVersion(device.appVersion).
		SetIconURL(device.iconURL).
		SetSupportsMediaControl(false).
		SetSupportsPersistentIdentifier(false).
		SetLastActivityAt(device.lastActivity).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the device %q: %v", device.name, err)
	}

	return clientID
}

func (f *fixture) addUser(t *testing.T, name string) *store.User {
	t.Helper()

	user, err := f.client.User.Create().
		SetName(name).
		SetUsername(f.prefix + name).
		SetPasswordHash("hash").
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the user %q: %v", name, err)
	}

	return user
}

func (f *fixture) addSession(t *testing.T, clientID string, user *store.User, lastActivity time.Time) uuid.UUID {
	t.Helper()

	device, err := f.client.Device.Query().Where(devicemodal.ClientID(clientID)).Only(context.Background())
	if err != nil {
		t.Fatalf("failed to find the device %q: %v", clientID, err)
	}

	session, err := f.client.Session.Create().
		SetUserID(user.ID).
		SetDeviceID(device.ID).
		SetAccessToken(uuid.NewString()).
		SetLastActivityAt(lastActivity).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	return session.ID
}

func (f *fixture) storedCustomName(t *testing.T, clientID string) string {
	t.Helper()

	device, err := f.client.Device.Query().Where(devicemodal.ClientID(clientID)).Only(context.Background())
	if err != nil {
		t.Fatalf("failed to find the device %q: %v", clientID, err)
	}

	return device.CustomName
}

func (f *fixture) mine(items []api.DeviceInfoDto) []api.DeviceInfoDto {
	found := make([]api.DeviceInfoDto, 0, len(items))
	for _, item := range items {
		if item.Id != nil && strings.HasPrefix(*item.Id, f.prefix) {
			found = append(found, item)
		}
	}

	return found
}

func TestServer_GetDevices(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	fixture.addDevice(t, deviceSeed{name: "Older", lastActivity: now.Add(-time.Hour)})
	fixture.addDevice(t, deviceSeed{name: "Newer", lastActivity: now})

	response, err := fixture.server.GetDevices(ctx, api.GetDevicesRequestObject{})
	if err != nil {
		t.Fatalf("failed to get the devices: %v", err)
	}

	result, ok := response.(api.GetDevices200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetDevices200JSONResponse", response)
	}

	items := fixture.mine(*result.Items)
	if len(items) != 2 {
		t.Fatalf("devices = %d, want 2", len(items))
	}
	if *items[0].Name != "Newer" || *items[1].Name != "Older" {
		t.Errorf("devices = %q, %q, want newest activity first", *items[0].Name, *items[1].Name)
	}
	if *result.TotalRecordCount != int32(len(*result.Items)) {
		t.Errorf("TotalRecordCount = %d, want %d", *result.TotalRecordCount, len(*result.Items))
	}
}

func TestServer_GetDeviceInfo(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	lastActivity := time.Now().UTC().Truncate(time.Millisecond)
	clientID := fixture.addDevice(t, deviceSeed{
		name:         "Living Room TV",
		customName:   "Big Screen",
		appName:      "Jellyfin Web",
		appVersion:   "10.10.0",
		iconURL:      "https://example.test/tv.png",
		lastActivity: lastActivity,
	})

	t.Run("maps the device onto the dto", func(t *testing.T) {
		response, err := fixture.server.GetDeviceInfo(ctx, api.GetDeviceInfoRequestObject{
			Params: api.GetDeviceInfoParams{Id: clientID},
		})
		if err != nil {
			t.Fatalf("failed to get the device info: %v", err)
		}

		info, ok := response.(api.GetDeviceInfo200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want api.GetDeviceInfo200JSONResponse", response)
		}

		if *info.Id != clientID {
			t.Errorf("Id = %q, want %q", *info.Id, clientID)
		}
		if *info.Name != "Living Room TV" {
			t.Errorf("Name = %q, want %q", *info.Name, "Living Room TV")
		}
		if *info.CustomName != "Big Screen" {
			t.Errorf("CustomName = %q, want %q", *info.CustomName, "Big Screen")
		}
		if *info.AppName != "Jellyfin Web" {
			t.Errorf("AppName = %q, want %q", *info.AppName, "Jellyfin Web")
		}
		if *info.AppVersion != "10.10.0" {
			t.Errorf("AppVersion = %q, want %q", *info.AppVersion, "10.10.0")
		}
		if *info.IconUrl != "https://example.test/tv.png" {
			t.Errorf("IconUrl = %q, want %q", *info.IconUrl, "https://example.test/tv.png")
		}
		if !info.DateLastActivity.Equal(lastActivity) {
			t.Errorf("DateLastActivity = %v, want %v", *info.DateLastActivity, lastActivity)
		}
		if info.LastUserId != nil {
			t.Errorf("LastUserId = %v, want nil", *info.LastUserId)
		}
	})

	t.Run("reports the user of the newest session", func(t *testing.T) {
		now := time.Now().UTC()
		dean := fixture.addUser(t, "Dean")
		older := fixture.addUser(t, "Older")
		fixture.addSession(t, clientID, older, now.Add(-time.Hour))
		fixture.addSession(t, clientID, dean, now)

		response, err := fixture.server.GetDeviceInfo(ctx, api.GetDeviceInfoRequestObject{
			Params: api.GetDeviceInfoParams{Id: clientID},
		})
		if err != nil {
			t.Fatalf("failed to get the device info: %v", err)
		}

		info, ok := response.(api.GetDeviceInfo200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want api.GetDeviceInfo200JSONResponse", response)
		}

		if info.LastUserId == nil || *info.LastUserId != dean.ID {
			t.Errorf("LastUserId = %v, want %v", info.LastUserId, dean.ID)
		}
		if info.LastUserName == nil || *info.LastUserName != "Dean" {
			t.Errorf("LastUserName = %v, want %q", info.LastUserName, "Dean")
		}
	})

	t.Run("404s for an unknown device", func(t *testing.T) {
		response, err := fixture.server.GetDeviceInfo(ctx, api.GetDeviceInfoRequestObject{
			Params: api.GetDeviceInfoParams{Id: "missing"},
		})
		if err != nil {
			t.Fatalf("failed to get the device info: %v", err)
		}

		if _, ok := response.(api.GetDeviceInfo404JSONResponse); !ok {
			t.Errorf("response = %T, want api.GetDeviceInfo404JSONResponse", response)
		}
	})
}

func TestServer_GetDeviceOptions(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	clientID := fixture.addDevice(t, deviceSeed{name: "TV", customName: "Big Screen"})

	t.Run("returns the custom name", func(t *testing.T) {
		response, err := fixture.server.GetDeviceOptions(ctx, api.GetDeviceOptionsRequestObject{
			Params: api.GetDeviceOptionsParams{Id: clientID},
		})
		if err != nil {
			t.Fatalf("failed to get the device options: %v", err)
		}

		options, ok := response.(api.GetDeviceOptions200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want api.GetDeviceOptions200JSONResponse", response)
		}

		if *options.DeviceId != clientID {
			t.Errorf("DeviceId = %q, want %q", *options.DeviceId, clientID)
		}
		if *options.CustomName != "Big Screen" {
			t.Errorf("CustomName = %q, want %q", *options.CustomName, "Big Screen")
		}
	})

	t.Run("404s for an unknown device", func(t *testing.T) {
		response, err := fixture.server.GetDeviceOptions(ctx, api.GetDeviceOptionsRequestObject{
			Params: api.GetDeviceOptionsParams{Id: "missing"},
		})
		if err != nil {
			t.Fatalf("failed to get the device options: %v", err)
		}

		if _, ok := response.(api.GetDeviceOptions404JSONResponse); !ok {
			t.Errorf("response = %T, want api.GetDeviceOptions404JSONResponse", response)
		}
	})
}

func TestServer_UpdateDeviceOptions(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	tests := []struct {
		name          string
		seed          deviceSeed
		body          *api.DeviceOptionsDto
		wantForbidden bool
		wantStored    string
	}{
		{
			name:       "writes the custom name",
			seed:       deviceSeed{name: "Renamed", customName: "Before"},
			body:       &api.DeviceOptionsDto{CustomName: ptr("After")},
			wantStored: "After",
		},
		{
			name:       "leaves the stored name alone when omitted",
			seed:       deviceSeed{name: "Untouched", customName: "Keep Me"},
			body:       &api.DeviceOptionsDto{},
			wantStored: "Keep Me",
		},
		{
			name: "clears the stored name when explicitly empty",
			seed: deviceSeed{name: "Cleared", customName: "Drop Me"},
			body: &api.DeviceOptionsDto{CustomName: ptr("")},
		},
		{
			name:          "refuses an empty body",
			seed:          deviceSeed{name: "NoBody", customName: "Keep Me"},
			body:          nil,
			wantForbidden: true,
			wantStored:    "Keep Me",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientID := fixture.addDevice(t, test.seed)

			response, err := fixture.server.UpdateDeviceOptions(ctx, api.UpdateDeviceOptionsRequestObject{
				Params:   api.UpdateDeviceOptionsParams{Id: clientID},
				JSONBody: test.body,
			})
			if err != nil {
				t.Fatalf("failed to update the device options: %v", err)
			}

			if test.wantForbidden {
				if !isUpdateForbidden(response) {
					t.Fatalf("response = %T, want api.UpdateDeviceOptions403Response", response)
				}
			} else if _, ok := response.(api.UpdateDeviceOptions204Response); !ok {
				t.Fatalf("response = %T, want api.UpdateDeviceOptions204Response", response)
			}

			if got := fixture.storedCustomName(t, clientID); got != test.wantStored {
				t.Errorf("CustomName = %q, want %q", got, test.wantStored)
			}
		})
	}
	t.Run("refuses an unknown device", func(t *testing.T) {
		fixture := newFixture(t)

		response, err := fixture.server.UpdateDeviceOptions(context.Background(), api.UpdateDeviceOptionsRequestObject{
			Params:   api.UpdateDeviceOptionsParams{Id: "missing"},
			JSONBody: &api.DeviceOptionsDto{CustomName: ptr("After")},
		})
		if err != nil {
			t.Fatalf("failed to update the device options: %v", err)
		}
		if !isUpdateForbidden(response) {
			t.Errorf("response = %T, want api.UpdateDeviceOptions403Response", response)
		}
	})

}

func TestServer_DeleteDevice(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	t.Run("removes the device and its sessions", func(t *testing.T) {
		clientID := fixture.addDevice(t, deviceSeed{name: "Doomed"})
		kept := fixture.addDevice(t, deviceSeed{name: "Kept"})
		dean := fixture.addUser(t, "Dean")
		fixture.addSession(t, clientID, dean, time.Now())
		keptSession := fixture.addSession(t, kept, dean, time.Now())

		response, err := fixture.server.DeleteDevice(ctx, api.DeleteDeviceRequestObject{
			Params: api.DeleteDeviceParams{Id: &[]string{clientID}},
		})
		if err != nil {
			t.Fatalf("failed to delete the device: %v", err)
		}
		if _, ok := response.(api.DeleteDevice204Response); !ok {
			t.Fatalf("response = %T, want api.DeleteDevice204Response", response)
		}

		devices, err := fixture.client.Device.Query().Where(devicemodal.ClientID(clientID)).Count(ctx)
		if err != nil {
			t.Fatalf("failed to count the devices: %v", err)
		}
		if devices != 0 {
			t.Errorf("devices = %d, want 0", devices)
		}

		orphans, err := fixture.client.Session.Query().
			Where(sessionmodal.HasDeviceWith(devicemodal.ClientID(clientID))).
			Count(ctx)
		if err != nil {
			t.Fatalf("failed to count the sessions: %v", err)
		}
		if orphans != 0 {
			t.Errorf("sessions = %d, want 0", orphans)
		}

		if exists, err := fixture.client.Session.Query().Where(sessionmodal.ID(keptSession)).Exist(ctx); err != nil {
			t.Fatalf("failed to look up the kept session: %v", err)
		} else if !exists {
			t.Error("the other device's session was deleted")
		}
	})

	t.Run("succeeds for an unknown device", func(t *testing.T) {
		response, err := fixture.server.DeleteDevice(ctx, api.DeleteDeviceRequestObject{
			Params: api.DeleteDeviceParams{Id: &[]string{"missing"}},
		})
		if err != nil {
			t.Fatalf("failed to delete the device: %v", err)
		}

		if _, ok := response.(api.DeleteDevice204Response); !ok {
			t.Errorf("response = %T, want api.DeleteDevice204Response", response)
		}
	})

	t.Run("rejects a request with no device ids", func(t *testing.T) {
		response, err := fixture.server.DeleteDevice(ctx, api.DeleteDeviceRequestObject{})
		if err != nil {
			t.Fatalf("failed to delete the device: %v", err)
		}

		if _, ok := response.(api.DeleteDevice400JSONResponse); !ok {
			t.Errorf("response = %T, want api.DeleteDevice400JSONResponse", response)
		}
	})
}

func isUpdateForbidden(response api.UpdateDeviceOptionsResponseObject) bool {
	_, ok := response.(api.UpdateDeviceOptions403Response)

	return ok
}

func ptr(value string) *string {
	return &value
}
