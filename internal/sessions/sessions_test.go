package sessions

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type fixture struct {
	service    *Service
	activities *activity.Service
	user       *store.User
	device     DeviceInfo
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

	ctx := context.Background()
	client := connection.Client()
	activities := activity.New(client)

	username := t.Name() + "-" + uuid.NewString()
	deviceID := uuid.NewString()

	user, err := client.User.Create().
		SetName(username).
		SetUsername(username).
		SetPasswordHash("").
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

	return &fixture{
		service:    New(client, activities),
		activities: activities,
		user:       user,
		device:     DeviceInfo{ID: deviceID, Name: "Firefox", AppName: "Jellyfin Web", AppVersion: "10.10.0"},
	}
}

func (f *fixture) entries(t *testing.T, since time.Time) []*activity.Entry {
	t.Helper()

	hasUser := true
	entries, _, err := f.activities.Entries(context.Background(), activity.Query{MinDate: &since, HasUserID: &hasUser})
	if err != nil {
		t.Fatalf("failed to query the entries: %v", err)
	}

	mine := make([]*activity.Entry, 0)
	for _, entry := range entries {
		if entry.Edges.User != nil && entry.Edges.User.ID == f.user.ID {
			mine = append(mine, entry)
		}
	}

	return mine
}

func TestService_Create(t *testing.T) {
	t.Run("records an authentication naming the user and the device", func(t *testing.T) {
		fixture := newFixture(t)
		token := uuid.NewString()

		start := time.Now()
		if _, err := fixture.service.Create(context.Background(), fixture.user.ID, token, fixture.device); err != nil {
			t.Fatalf("failed to create the session: %v", err)
		}

		entries := fixture.entries(t, start)
		if len(entries) != 1 {
			t.Fatalf("entries = %d, want 1", len(entries))
		}

		entry := entries[0]
		if entry.Kind != activity.KindAuthenticationSucceeded {
			t.Errorf("kind = %q, want %q", entry.Kind, activity.KindAuthenticationSucceeded)
		}
		if !strings.Contains(entry.Name, fixture.user.Username) {
			t.Errorf("name = %q, want it to name %q", entry.Name, fixture.user.Username)
		}
		if entry.ShortOverview != fixture.device.Name {
			t.Errorf("short overview = %q, want %q", entry.ShortOverview, fixture.device.Name)
		}
		for field, value := range map[string]string{"name": entry.Name, "overview": entry.Overview, "short overview": entry.ShortOverview} {
			if strings.Contains(value, token) {
				t.Errorf("%s leaks the access token: %q", field, value)
			}
		}
	})
}

func TestService_DeleteByToken(t *testing.T) {
	t.Run("records the disconnect naming the user and the device", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		token := uuid.NewString()

		if _, err := fixture.service.Create(ctx, fixture.user.ID, token, fixture.device); err != nil {
			t.Fatalf("failed to create the session: %v", err)
		}

		start := time.Now()
		if err := fixture.service.DeleteByToken(ctx, token); err != nil {
			t.Fatalf("failed to delete the session: %v", err)
		}

		entries := fixture.entries(t, start)
		if len(entries) != 1 {
			t.Fatalf("entries = %d, want 1", len(entries))
		}

		entry := entries[0]
		if entry.Kind != activity.KindSessionEnded {
			t.Errorf("kind = %q, want %q", entry.Kind, activity.KindSessionEnded)
		}
		if !strings.Contains(entry.Name, fixture.user.Username) {
			t.Errorf("name = %q, want it to name %q", entry.Name, fixture.user.Username)
		}
		if entry.ShortOverview != fixture.device.Name {
			t.Errorf("short overview = %q, want %q", entry.ShortOverview, fixture.device.Name)
		}
		for field, value := range map[string]string{"name": entry.Name, "overview": entry.Overview, "short overview": entry.ShortOverview} {
			if strings.Contains(value, token) {
				t.Errorf("%s leaks the access token: %q", field, value)
			}
		}
	})

	t.Run("succeeds for a token that is already gone", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		token := uuid.NewString()

		if _, err := fixture.service.Create(ctx, fixture.user.ID, token, fixture.device); err != nil {
			t.Fatalf("failed to create the session: %v", err)
		}
		if err := fixture.service.DeleteByToken(ctx, token); err != nil {
			t.Fatalf("failed to delete the session: %v", err)
		}

		start := time.Now()
		if err := fixture.service.DeleteByToken(ctx, token); err != nil {
			t.Fatalf("a second logout failed, which the client reads as a 500: %v", err)
		}

		if entries := fixture.entries(t, start); len(entries) != 0 {
			t.Errorf("entries = %d, want 0 for a session that was already gone", len(entries))
		}
	})
}
