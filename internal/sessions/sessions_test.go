package sessions

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type fixture struct {
	client     *store.Client
	service    *Service
	activities *activity.Service
	user       *User
	device     Device
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection := storetest.Open(t)

	ctx := context.Background()
	client := connection.Client()
	activities := activity.New(client)

	username := t.Name() + "-" + uuid.NewString()
	deviceID := uuid.NewString()

	user, err := client.User.Create().
		SetName(username).
		SetUsername(username).
		SetPasswordHash("").
		SaveModel(ctx)
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
		if err := client.User.DeleteOneID(user.ID).Exec(ctx); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
	})

	return &fixture{
		client:     client,
		service:    New(client, activities),
		activities: activities,
		user:       user,
		device:     Device{ClientID: deviceID, Name: "Firefox", AppName: "Jellyfin Web", AppVersion: "10.10.0"},
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
		if entry.User != nil && entry.User.ID == f.user.ID {
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

func TestService_Playing(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	client := fixture.service.store
	key := uuid.NewString()

	series := client.Item.Create().SetKind(itemmodel.KindSeries).SetKey("series:" + key).SetName("Fixture Show").SaveX(ctx)
	season := client.Item.Create().SetKind(itemmodel.KindSeason).SetKey("season:" + key).SetName("Season 1").SetParentID(series.ID).SaveX(ctx)
	episode := client.Item.Create().SetKind(itemmodel.KindEpisode).SetKey("episode:" + key).SetName("Pilot").SetParentID(season.ID).SaveX(ctx)
	t.Cleanup(func() {
		if _, err := client.Item.Delete().Where(itemmodel.IDIn(episode.ID, season.ID, series.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
	})

	session, err := fixture.service.Create(ctx, fixture.user.ID, uuid.NewString(), fixture.device)
	if err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	listed := func() *Session {
		t.Helper()
		all, err := fixture.service.List(ctx)
		if err != nil {
			t.Fatalf("failed to list the sessions: %v", err)
		}
		for _, candidate := range all {
			if candidate.ID == session.ID {
				return candidate
			}
		}
		t.Fatalf("session %s was not listed", session.ID)

		return nil
	}

	playback := Playback{ItemID: episode.ID, PositionTicks: 42, PlayMethod: "Transcode", IsPaused: true}
	if err := fixture.service.Playing(ctx, session.ID, "203.0.113.7:51234", playback); err != nil {
		t.Fatalf("failed to record playback: %v", err)
	}

	playing := listed()
	if !NowPlaying(playing, time.Now()) {
		t.Fatalf("NowPlaying = false after a progress report")
	}
	if playing.NowPlaying.Parent == nil || playing.NowPlaying.Parent.Parent == nil || playing.NowPlaying.Parent.Parent.Name != "Fixture Show" {
		t.Errorf("the playing episode's series was not loaded")
	}
	if playing.PositionTicks != 42 || playing.PlayMethod != "Transcode" || !playing.IsPaused {
		t.Errorf("state = %d %q %v, want 42 Transcode paused", playing.PositionTicks, playing.PlayMethod, playing.IsPaused)
	}
	if playing.RemoteEndpoint != "203.0.113.7" {
		t.Errorf("RemoteEndpoint = %q, want the address without its port", playing.RemoteEndpoint)
	}
	if NowPlaying(playing, time.Now().Add(PlayingTimeout)) {
		t.Errorf("NowPlaying = true for a session that has not reported in %s", PlayingTimeout)
	}

	if err := fixture.service.Stopped(ctx, session.ID, "203.0.113.7:51234"); err != nil {
		t.Fatalf("failed to record the stop: %v", err)
	}
	if stopped := listed(); NowPlaying(stopped, time.Now()) || stopped.NowPlayingItemID != nil {
		t.Errorf("the session still says it is playing after a stop")
	}
}

func (f *fixture) listed(t *testing.T, id uuid.UUID) *Session {
	t.Helper()

	all, err := f.service.List(context.Background())
	if err != nil {
		t.Fatalf("failed to list the sessions: %v", err)
	}
	for _, candidate := range all {
		if candidate.ID == id {
			return candidate
		}
	}
	t.Fatalf("session %s was not listed", id)

	return nil
}

func TestService_Viewing(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	client := fixture.service.store

	movie := client.Item.Create().SetKind(itemmodel.KindMovie).SetKey("movie:" + uuid.NewString()).SetName("Fixture Movie").SaveX(ctx)
	t.Cleanup(func() {
		if err := client.Item.DeleteOneID(movie.ID).Exec(ctx); err != nil {
			t.Errorf("failed to delete the item: %v", err)
		}
	})

	session, err := fixture.service.Create(ctx, fixture.user.ID, uuid.NewString(), fixture.device)
	if err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	if err := fixture.service.Viewing(ctx, session.ID, uuid.New()); !errors.Is(err, ErrNoSuchItem) {
		t.Errorf("viewing an unknown item = %v, want ErrNoSuchItem", err)
	}

	if err := fixture.service.Viewing(ctx, session.ID, movie.ID); err != nil {
		t.Fatalf("failed to record what is being viewed: %v", err)
	}

	viewing := fixture.listed(t, session.ID)
	if !NowViewing(viewing, time.Now()) || viewing.NowViewing.Name != "Fixture Movie" {
		t.Errorf("NowViewing = false or the wrong item after a report")
	}
	if NowViewing(viewing, time.Now().Add(ViewingTimeout)) {
		t.Errorf("NowViewing = true for a session that has not reported in %s", ViewingTimeout)
	}
}

func TestService_AdditionalUsers(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	client := fixture.service.store

	name := "guest-" + uuid.NewString()
	guest := client.User.Create().SetName(name).SetUsername(name).SetPasswordHash("").SaveX(ctx)
	t.Cleanup(func() {
		if err := client.User.DeleteOneID(guest.ID).Exec(ctx); err != nil {
			t.Errorf("failed to delete the guest: %v", err)
		}
	})

	session, err := fixture.service.Create(ctx, fixture.user.ID, uuid.NewString(), fixture.device)
	if err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	if err := fixture.service.AddUser(ctx, session.ID, fixture.user.ID); !errors.Is(err, ErrPrimaryUser) {
		t.Errorf("adding the owner = %v, want ErrPrimaryUser", err)
	}
	if err := fixture.service.AddUser(ctx, session.ID, uuid.New()); !errors.Is(err, ErrNoUser) {
		t.Errorf("adding nobody = %v, want ErrNoUser", err)
	}
	if err := fixture.service.AddUser(ctx, uuid.New(), guest.ID); !errors.Is(err, ErrNoSession) {
		t.Errorf("adding to no session = %v, want ErrNoSession", err)
	}

	for range 2 {
		if err := fixture.service.AddUser(ctx, session.ID, guest.ID); err != nil {
			t.Fatalf("failed to add the guest: %v", err)
		}
	}
	if users := fixture.listed(t, session.ID).AdditionalUsers; len(users) != 1 || users[0].ID != guest.ID {
		t.Errorf("additional users = %v, want the guest once", users)
	}

	if err := fixture.service.RemoveUser(ctx, session.ID, guest.ID); err != nil {
		t.Fatalf("failed to remove the guest: %v", err)
	}
	if users := fixture.listed(t, session.ID).AdditionalUsers; len(users) != 0 {
		t.Errorf("additional users = %v, want none after the guest left", users)
	}
}

func TestService_ReportCapabilities(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	token := uuid.NewString()

	if _, err := fixture.service.Create(ctx, fixture.user.ID, token, fixture.device); err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}
	session, err := fixture.service.ByToken(ctx, token)
	if err != nil {
		t.Fatalf("failed to read the session back: %v", err)
	}

	profile := map[string]any{"Name": "Example Player"}
	if err := fixture.service.ReportCapabilities(ctx, session, Capabilities{
		PlayableMediaTypes:   []string{"Video"},
		SupportedCommands:    []string{"Play"},
		SupportsMediaControl: true,
		Profile:              profile,
	}); err != nil {
		t.Fatalf("failed to report the capabilities: %v", err)
	}

	reported, err := fixture.service.ByToken(ctx, token)
	if err != nil {
		t.Fatalf("failed to read the session back: %v", err)
	}
	if got := Profile(reported); got["Name"] != "Example Player" {
		t.Errorf("profile = %v, want the one the device reported", got)
	}
	if !reported.Device.SupportsMediaControl || len(reported.Device.PlayableMediaTypes) != 1 || len(reported.Device.SupportedCommands) != 1 {
		t.Errorf("device = %+v, want the reported capabilities", reported.Device)
	}

	if err := fixture.service.ReportCapabilities(ctx, reported, Capabilities{}); err != nil {
		t.Fatalf("failed to report the capabilities again: %v", err)
	}
	if again, err := fixture.service.ByToken(ctx, token); err != nil {
		t.Fatalf("failed to read the session back: %v", err)
	} else if Profile(again) != nil {
		t.Errorf("profile = %v, want it cleared by a report that carried none", Profile(again))
	}
}
