package sessions

import (
	"context"
	"slices"
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

func TestSessionsRecordActivity(t *testing.T) {
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
	service := New(client, activities)

	username := t.Name() + "-" + uuid.NewString()
	deviceID := uuid.NewString()
	token := uuid.NewString()

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

	start := time.Now()
	device := DeviceInfo{ID: deviceID, Name: "Firefox", AppName: "Jellyfin Web", AppVersion: "10.10.0"}
	if _, err := service.Create(ctx, user.ID, token, device); err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}
	if err := service.DeleteByToken(ctx, token); err != nil {
		t.Fatalf("failed to delete the session: %v", err)
	}

	hasUser := true
	entries, _, err := activities.Entries(ctx, activity.Query{MinDate: &start, HasUserID: &hasUser})
	if err != nil {
		t.Fatalf("failed to query the entries: %v", err)
	}

	kinds := make([]string, 0)
	for _, entry := range entries {
		if entry.Edges.User == nil || entry.Edges.User.ID != user.ID {
			continue
		}
		kinds = append(kinds, entry.Kind)

		if !strings.Contains(entry.Name, username) {
			t.Errorf("name = %q, want it to name %q", entry.Name, username)
		}
		if entry.ShortOverview != device.Name {
			t.Errorf("short overview = %q, want %q", entry.ShortOverview, device.Name)
		}
		for field, value := range map[string]string{"name": entry.Name, "overview": entry.Overview, "short overview": entry.ShortOverview} {
			if strings.Contains(value, token) {
				t.Errorf("%s leaks the access token: %q", field, value)
			}
		}
	}

	slices.Sort(kinds)
	want := []string{activity.KindAuthenticationSucceeded, activity.KindSessionEnded}
	if !slices.Equal(kinds, want) {
		t.Errorf("kinds = %v, want %v", kinds, want)
	}
}
