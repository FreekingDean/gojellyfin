package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

const importedHash = "$PBKDF2-SHA512$iterations=210000$0102030405060708090A0B0C0D0E0F10$" +
	"8A5A90C00EB2155E81D3C6B82ABEE6B9875A0E7CE286688E796A32F9675D1724" +
	"5E71EE6B008F47256E6EC66BBD8040417DF06E50007501FEDF44A2588B949C1F"

const jellyfinSchema = `
CREATE TABLE Users (
	Id TEXT NOT NULL PRIMARY KEY, AudioLanguagePreference TEXT,
	AuthenticationProviderId TEXT NOT NULL, CastReceiverId TEXT,
	DisplayCollectionsView INTEGER NOT NULL, DisplayMissingEpisodes INTEGER NOT NULL,
	EnableAutoLogin INTEGER NOT NULL, EnableLocalPassword INTEGER NOT NULL,
	EnableNextEpisodeAutoPlay INTEGER NOT NULL, EnableUserPreferenceAccess INTEGER NOT NULL,
	HidePlayedInLatest INTEGER NOT NULL, InternalId INTEGER NOT NULL,
	InvalidLoginAttemptCount INTEGER NOT NULL, LastActivityDate TEXT, LastLoginDate TEXT,
	LoginAttemptsBeforeLockout INTEGER, MaxActiveSessions INTEGER NOT NULL,
	MaxParentalAgeRating INTEGER, MustUpdatePassword INTEGER NOT NULL, Password TEXT,
	PasswordResetProviderId TEXT NOT NULL, PlayDefaultAudioTrack INTEGER NOT NULL,
	RememberAudioSelections INTEGER NOT NULL, RememberSubtitleSelections INTEGER NOT NULL,
	RemoteClientBitrateLimit INTEGER, RowVersion INTEGER NOT NULL,
	SubtitleLanguagePreference TEXT, SubtitleMode INTEGER NOT NULL,
	SyncPlayAccess INTEGER NOT NULL, Username TEXT NOT NULL COLLATE NOCASE);
CREATE TABLE Permissions (
	Id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, Kind INTEGER NOT NULL,
	Permission_Permissions_Guid TEXT, RowVersion INTEGER NOT NULL, UserId TEXT,
	Value INTEGER NOT NULL);
CREATE TABLE Devices (
	Id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, AccessToken TEXT NOT NULL,
	AppName TEXT NOT NULL, AppVersion TEXT NOT NULL, DateCreated TEXT NOT NULL,
	DateLastActivity TEXT NOT NULL, DateModified TEXT NOT NULL, DeviceId TEXT NOT NULL,
	DeviceName TEXT NOT NULL, IsActive INTEGER NOT NULL, UserId TEXT NOT NULL);
`

type importFixture struct {
	client *store.Client
	source *sql.DB
	from   string
	prefix string
}

func newImportFixture(t *testing.T) *importFixture {
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
	prefix := strings.ReplaceAll(t.Name(), "/", "-") + "-" + uuid.NewString() + "-"

	from := t.TempDir()
	source, err := sql.Open("sqlite", filepath.Join(from, jellyfinDatabase))
	if err != nil {
		t.Fatalf("failed to create the source database: %v", err)
	}
	if _, err := source.Exec(jellyfinSchema); err != nil {
		t.Fatalf("failed to create the source schema: %v", err)
	}

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
		if _, err := client.User.Delete().
			Where(usermodal.UsernameHasPrefix(prefix)).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
		if err := source.Close(); err != nil {
			t.Errorf("failed to close the source database: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &importFixture{client: client, source: source, from: from, prefix: prefix}
}

func (f *importFixture) addUser(t *testing.T, name, password string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := f.source.Exec(`
		INSERT INTO Users (Id, Username, Password, AuthenticationProviderId, PasswordResetProviderId,
			DisplayCollectionsView, DisplayMissingEpisodes, EnableAutoLogin, EnableLocalPassword,
			EnableNextEpisodeAutoPlay, EnableUserPreferenceAccess, HidePlayedInLatest, InternalId,
			InvalidLoginAttemptCount, MaxActiveSessions, MustUpdatePassword, PlayDefaultAudioTrack,
			RememberAudioSelections, RememberSubtitleSelections, RowVersion, SubtitleMode, SyncPlayAccess)
		VALUES (?, ?, ?, 'Default', 'Default', 0, 0, 0, 0, 1, 1, 1, 0, 0, 0, 0, 1, 1, 1, 1, 2, 0)`,
		id.String(), f.prefix+name, password)
	if err != nil {
		t.Fatalf("failed to seed the user %q: %v", name, err)
	}

	return id
}

func (f *importFixture) addPermission(t *testing.T, user uuid.UUID, kind int, value bool) {
	t.Helper()

	_, err := f.source.Exec(`INSERT INTO Permissions (Kind, RowVersion, UserId, Value) VALUES (?, 1, ?, ?)`,
		kind, user.String(), value)
	if err != nil {
		t.Fatalf("failed to seed a permission: %v", err)
	}
}

func (f *importFixture) addDevice(t *testing.T, user uuid.UUID, name, token string, lastActivity time.Time, active bool) {
	t.Helper()

	stamp := lastActivity.UTC().Format("2006-01-02 15:04:05.9999999")
	_, err := f.source.Exec(`
		INSERT INTO Devices (AccessToken, AppName, AppVersion, DateCreated, DateLastActivity,
			DateModified, DeviceId, DeviceName, IsActive, UserId)
		VALUES (?, 'Test Client', '1.2.3', ?, ?, ?, ?, ?, ?, ?)`,
		token, stamp, stamp, stamp, f.prefix+name, name, active, user.String())
	if err != nil {
		t.Fatalf("failed to seed the device %q: %v", name, err)
	}
}

func (f *importFixture) run(t *testing.T, activeDays int, dryRun bool) *importReport {
	t.Helper()

	source, err := openReadOnly(filepath.Join(f.from, jellyfinDatabase))
	if err != nil {
		t.Fatalf("failed to open the source: %v", err)
	}
	defer func() { _ = source.Close() }()

	report, err := runImport(context.Background(), source, f.client, activeDays, dryRun)
	if err != nil {
		t.Fatalf("failed to import: %v", err)
	}

	return report
}

func (f *importFixture) user(t *testing.T, name string) *store.User {
	t.Helper()

	user, err := users.New(f.client).UserByUsername(context.Background(), f.prefix+name)
	if err != nil {
		t.Fatalf("failed to read the user %q: %v", name, err)
	}

	return user
}

func TestRunImport(t *testing.T) {
	t.Run("creates a user with the password hash the source held", func(t *testing.T) {
		fixture := newImportFixture(t)
		fixture.addUser(t, "viewer", importedHash)

		report := fixture.run(t, 0, false)
		if len(report.created) != 1 {
			t.Fatalf("created = %v, want one user", report.created)
		}

		user := fixture.user(t, "viewer")
		matches, err := auth.Verify("hunter2", user.PasswordHash)
		if err != nil {
			t.Fatalf("failed to verify the imported hash: %v", err)
		}
		if !matches {
			t.Error("the imported password no longer signs the user in")
		}
	})

	t.Run("reports a user the source left without a password", func(t *testing.T) {
		fixture := newImportFixture(t)
		fixture.addUser(t, "guest", "")

		report := fixture.run(t, 0, false)
		if len(report.needingReset) != 1 {
			t.Fatalf("needingReset = %v, want one user", report.needingReset)
		}

		if matches, _ := auth.Verify("", fixture.user(t, "guest").PasswordHash); matches {
			t.Error("a user with no password can sign in with an empty one")
		}
	})

	t.Run("carries a permission the source had turned off", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "restricted", importedHash)
		fixture.addPermission(t, id, permissionIsAdministrator, false)
		fixture.addPermission(t, id, permissionEnableRemoteAccess, false)
		fixture.addPermission(t, id, permissionEnableContentDownloading, false)

		fixture.run(t, 0, false)

		policy, err := fixture.user(t, "restricted").QueryPolicy().Only(context.Background())
		if err != nil {
			t.Fatalf("failed to read the policy: %v", err)
		}
		if policy.IsAdministrator {
			t.Error("a non-administrator was imported as one")
		}
		if policy.EnableRemoteAccess {
			t.Error("remote access was turned off in the source and is on here")
		}
		if policy.EnableContentDownloading {
			t.Error("downloading was turned off in the source and is on here")
		}
	})

	t.Run("makes an administrator an administrator", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "owner", importedHash)
		fixture.addPermission(t, id, permissionIsAdministrator, true)

		fixture.run(t, 0, false)

		policy, err := fixture.user(t, "owner").QueryPolicy().Only(context.Background())
		if err != nil {
			t.Fatalf("failed to read the policy: %v", err)
		}
		if !policy.IsAdministrator {
			t.Error("an administrator was imported without the rights")
		}
	})

	t.Run("keeps a device signed in on the token it already had", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "viewer", importedHash)
		token := uuid.NewString()
		fixture.addDevice(t, id, "Television", token, time.Now(), true)

		report := fixture.run(t, 0, false)
		if report.sessions != 1 {
			t.Fatalf("sessions = %d, want 1", report.sessions)
		}

		session, err := sessions.New(fixture.client).ByToken(context.Background(), token)
		if err != nil {
			t.Fatalf("the imported token does not sign in: %v", err)
		}
		if session.Edges.User.Username != fixture.prefix+"viewer" {
			t.Errorf("the session landed on %q", session.Edges.User.Username)
		}
		if session.Edges.Device.Name != "Television" {
			t.Errorf("device name = %q, want Television", session.Edges.Device.Name)
		}
	})

	t.Run("skips a device the source had already signed out", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "viewer", importedHash)
		fixture.addDevice(t, id, "Old Phone", uuid.NewString(), time.Now(), false)

		if report := fixture.run(t, 0, false); report.sessions != 0 {
			t.Errorf("sessions = %d, want none: an inactive device was imported", report.sessions)
		}
	})

	t.Run("skips a device last seen before the cutoff", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "viewer", importedHash)
		fixture.addDevice(t, id, "Recent", uuid.NewString(), time.Now().AddDate(0, 0, -3), true)
		fixture.addDevice(t, id, "Forgotten", uuid.NewString(), time.Now().AddDate(-1, 0, 0), true)

		report := fixture.run(t, 30, false)
		if report.sessions != 1 {
			t.Errorf("sessions = %d, want 1", report.sessions)
		}
		if report.stale != 1 {
			t.Errorf("stale = %d, want 1", report.stale)
		}
	})

	t.Run("reports a device whose user the source no longer holds", func(t *testing.T) {
		fixture := newImportFixture(t)
		fixture.addDevice(t, uuid.New(), "Stranger", uuid.NewString(), time.Now(), true)

		report := fixture.run(t, 0, false)
		if len(report.orphaned) != 1 || report.orphaned[0] != "Stranger" {
			t.Errorf("orphaned = %v, want the one device", report.orphaned)
		}
		if report.sessions != 0 {
			t.Errorf("sessions = %d, want none", report.sessions)
		}
	})

	t.Run("imports nothing twice", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "viewer", importedHash)
		fixture.addDevice(t, id, "Television", uuid.NewString(), time.Now(), true)

		fixture.run(t, 0, false)
		before := fixture.user(t, "viewer")

		second := fixture.run(t, 0, false)
		if len(second.created) != 0 {
			t.Errorf("created = %v, want none on the second run", second.created)
		}
		if len(second.existing) != 1 {
			t.Errorf("existing = %v, want the one user", second.existing)
		}

		after := fixture.user(t, "viewer")
		if after.ID != before.ID {
			t.Error("the second run replaced the user rather than recognising them")
		}

		count, err := fixture.client.Session.Query().
			Where(sessionmodal.HasDeviceWith(devicemodal.ClientIDHasPrefix(fixture.prefix))).
			Count(context.Background())
		if err != nil {
			t.Fatalf("failed to count the sessions: %v", err)
		}
		if count != 1 {
			t.Errorf("sessions = %d, want 1: the second run made another", count)
		}
	})

	t.Run("leaves a user who is already here alone", func(t *testing.T) {
		fixture := newImportFixture(t)
		existing, err := users.New(fixture.client).
			CreateUser(context.Background(), fixture.prefix+"viewer", "already-set", false)
		if err != nil {
			t.Fatalf("failed to seed the existing user: %v", err)
		}
		fixture.addUser(t, "viewer", importedHash)

		report := fixture.run(t, 0, false)
		if len(report.existing) != 1 {
			t.Fatalf("existing = %v, want the one user", report.existing)
		}

		after := fixture.user(t, "viewer")
		if after.ID != existing.ID || after.PasswordHash != "already-set" {
			t.Error("the import overwrote a user who was already here")
		}
	})

	t.Run("refuses to hand a local account the source's tokens", func(t *testing.T) {
		fixture := newImportFixture(t)
		if _, err := users.New(fixture.client).
			CreateUser(context.Background(), fixture.prefix+"shared", "already-set", true); err != nil {
			t.Fatalf("failed to seed the existing user: %v", err)
		}

		id := fixture.addUser(t, "shared", importedHash)
		fixture.addPermission(t, id, permissionIsAdministrator, false)
		token := uuid.NewString()
		fixture.addDevice(t, id, "Television", token, time.Now(), true)

		report := fixture.run(t, 0, false)
		if report.sessions != 0 {
			t.Errorf("sessions = %d, want none", report.sessions)
		}
		if len(report.claimed) != 1 || report.claimed[0] != "Television" {
			t.Errorf("claimed = %v, want the one device", report.claimed)
		}
		if _, err := sessions.New(fixture.client).ByToken(context.Background(), token); err == nil {
			t.Error("a token from the source signs in as the local account of the same name")
		}
	})

	t.Run("leaves a session revoked here revoked", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "viewer", importedHash)
		token := uuid.NewString()
		fixture.addDevice(t, id, "Television", token, time.Now(), true)

		fixture.run(t, 0, false)

		ctx := context.Background()
		revoked, err := fixture.client.Session.Update().
			Where(sessionmodal.AccessToken(token)).
			SetRevokedAt(time.Now()).
			Save(ctx)
		if err != nil || revoked != 1 {
			t.Fatalf("failed to revoke the session: %v", err)
		}

		fixture.run(t, 0, false)

		if _, err := sessions.New(fixture.client).ByToken(ctx, token); err == nil {
			t.Error("a second import brought a revoked token back to life")
		}
	})

	t.Run("refuses a device whose timestamp it cannot read", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "viewer", importedHash)
		if _, err := fixture.source.Exec(`
			INSERT INTO Devices (AccessToken, AppName, AppVersion, DateCreated, DateLastActivity,
				DateModified, DeviceId, DeviceName, IsActive, UserId)
			VALUES (?, 'Test Client', '1.2.3', '', 'the day before yesterday', '', ?, 'Broken', 1, ?)`,
			uuid.NewString(), fixture.prefix+"broken", id.String()); err != nil {
			t.Fatalf("failed to seed the device: %v", err)
		}

		source, err := openReadOnly(filepath.Join(fixture.from, jellyfinDatabase))
		if err != nil {
			t.Fatalf("failed to open the source: %v", err)
		}
		defer func() { _ = source.Close() }()

		if _, err := runImport(context.Background(), source, fixture.client, 0, true); err == nil {
			t.Error("an unreadable timestamp was folded into the report rather than refused")
		}
	})

	t.Run("writes nothing on a dry run", func(t *testing.T) {
		fixture := newImportFixture(t)
		id := fixture.addUser(t, "viewer", importedHash)
		token := uuid.NewString()
		fixture.addDevice(t, id, "Television", token, time.Now(), true)

		report := fixture.run(t, 0, true)
		if len(report.created) != 1 || report.sessions != 1 {
			t.Errorf("report = %+v, want one user and one session", report)
		}

		ctx := context.Background()
		count, err := fixture.client.User.Query().
			Where(usermodal.UsernameHasPrefix(fixture.prefix)).
			Count(ctx)
		if err != nil {
			t.Fatalf("failed to count the users: %v", err)
		}
		if count != 0 {
			t.Errorf("users = %d, want none: a dry run wrote rows", count)
		}
		if _, err := sessions.New(fixture.client).ByToken(ctx, token); err == nil {
			t.Error("a dry run wrote a session")
		}
	})

	t.Run("leaves the source database untouched", func(t *testing.T) {
		fixture := newImportFixture(t)
		fixture.addUser(t, "viewer", importedHash)

		source, err := openReadOnly(filepath.Join(fixture.from, jellyfinDatabase))
		if err != nil {
			t.Fatalf("failed to open the source: %v", err)
		}
		defer func() { _ = source.Close() }()

		if _, err := source.Exec(`UPDATE Users SET Password = 'rewritten'`); err == nil {
			t.Error("the source database accepted a write")
		}
	})
}

func TestImportReport_Lines(t *testing.T) {
	report := &importReport{
		created:      []string{"one"},
		existing:     []string{"two"},
		needingReset: []string{"one"},
		sessions:     3,
		stale:        2,
		orphaned:     []string{"Stranger"},
	}

	t.Run("names everything it skipped", func(t *testing.T) {
		written := strings.Join(report.lines(false), "\n")

		for _, want := range []string{"resetpassword", "one", "two", "Stranger", "skipped 2"} {
			if !strings.Contains(written, want) {
				t.Errorf("the report does not mention %q:\n%s", want, written)
			}
		}
	})

	t.Run("says nothing happened on a dry run", func(t *testing.T) {
		written := strings.Join(report.lines(true), "\n")

		if !strings.Contains(written, "would import") {
			t.Errorf("a dry run reads as though it wrote:\n%s", written)
		}
	})
}
