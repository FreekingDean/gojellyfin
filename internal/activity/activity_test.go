package activity

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type fixture struct {
	service *Service
	client  *store.Client
	userID  uuid.UUID
	future  time.Time
	ids     []uuid.UUID
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
	user, err := client.User.Create().
		SetName(t.Name()).
		SetUsername(t.Name() + "-" + uuid.NewString()).
		SetPasswordHash("").
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}

	f := &fixture{
		service: New(client),
		client:  client,
		userID:  user.ID,
		future:  time.Now().Add(time.Hour).Truncate(time.Millisecond),
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.ActivityLogEntry.Delete().Where(entrymodel.IDIn(f.ids...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the entries: %v", err)
		}
		if err := client.User.DeleteOne(user).Exec(ctx); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return f
}

func (f *fixture) add(t *testing.T, name string, at time.Time, userID *uuid.UUID) {
	t.Helper()

	entry, err := f.client.ActivityLogEntry.Create().
		SetName(name).
		SetKind(KindLibraryScanCompleted).
		SetShortOverview("seeded").
		SetSeverity(SeverityInformation).
		SetCreatedAt(at).
		SetNillableUserID(userID).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", name, err)
	}

	f.ids = append(f.ids, entry.ID)
}

func (f *fixture) names(t *testing.T, query Query) ([]string, int) {
	t.Helper()

	entries, total, err := f.service.Entries(context.Background(), query)
	if err != nil {
		t.Fatalf("failed to read the entries: %v", err)
	}

	found := make([]string, 0, len(entries))
	for _, entry := range entries {
		found = append(found, entry.Name)
	}

	return found, total
}

func TestService_Entries(t *testing.T) {
	fixture := newFixture(t)

	fixture.add(t, "Oldest", fixture.future, nil)
	fixture.add(t, "Middle", fixture.future.Add(time.Minute), nil)
	fixture.add(t, "Newest", fixture.future.Add(2*time.Minute), nil)
	fixture.add(t, "By User", fixture.future.Add(3*time.Minute), &fixture.userID)

	minDate := fixture.future.Add(2 * time.Minute)
	maxDate := fixture.future.Add(time.Minute)
	withUser := true
	withoutUser := false

	tests := []struct {
		name  string
		query Query
		want  []string
		total int
	}{
		{"returns the newest entry first", Query{MinDate: &fixture.future}, []string{"By User", "Newest", "Middle", "Oldest"}, 4},
		{"pages without changing the total", Query{MinDate: &fixture.future, StartIndex: 1, Limit: 2}, []string{"Newest", "Middle"}, 4},
		{"drops entries before the minimum date", Query{MinDate: &minDate}, []string{"By User", "Newest"}, 2},
		{"bounds the window at both ends", Query{MinDate: &fixture.future, MaxDate: &maxDate}, []string{"Middle", "Oldest"}, 2},
		{"filters on having a user", Query{MinDate: &fixture.future, HasUserID: &withUser}, []string{"By User"}, 1},
		{"filters on not having a user", Query{MinDate: &fixture.future, HasUserID: &withoutUser}, []string{"Newest", "Middle", "Oldest"}, 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, total := fixture.names(t, test.query)
			if !slices.Equal(got, test.want) {
				t.Errorf("entries = %v, want %v", got, test.want)
			}
			if total != test.total {
				t.Errorf("total = %d, want %d", total, test.total)
			}
		})
	}
}

func TestService_EntriesFilters(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	username, err := fixture.client.User.Query().Where(usermodel.ID(fixture.userID)).OnlyModel(ctx)
	if err != nil {
		t.Fatalf("failed to read the user: %v", err)
	}

	seed := func(name, kind, overview string, severity Severity, at time.Duration, userID *uuid.UUID) {
		entry, err := fixture.client.ActivityLogEntry.Create().
			SetName(name).
			SetKind(kind).
			SetOverview(overview).
			SetShortOverview("short " + name).
			SetSeverity(severity).
			SetCreatedAt(fixture.future.Add(at)).
			SetNillableUserID(userID).
			Save(ctx)
		if err != nil {
			t.Fatalf("failed to create %q: %v", name, err)
		}
		fixture.ids = append(fixture.ids, entry.ID)
	}

	seed("Alpha Scan", KindLibraryScanCompleted, "Scanned the films", entrymodel.SeverityInformation, 0, nil)
	seed("Bravo Login", KindAuthenticationSucceeded, "Signed in from a phone", entrymodel.SeverityWarning, time.Minute, &fixture.userID)
	seed("Charlie Error", KindLibraryScanCompleted, "Scan failed", entrymodel.SeverityError, 2*time.Minute, nil)

	warning := entrymodel.SeverityWarning
	tests := []struct {
		name  string
		query Query
		want  []string
	}{
		{"matches part of a name in any case", Query{Name: "SCAN"}, []string{"Alpha Scan"}},
		{"matches part of an overview", Query{Overview: "phone"}, []string{"Bravo Login"}},
		{"matches part of a short overview", Query{ShortOverview: "charlie"}, []string{"Charlie Error"}},
		{"matches part of a type", Query{Kind: "authentication"}, []string{"Bravo Login"}},
		{"matches part of the username", Query{Username: strings.ToUpper(username.Username[:6])}, []string{"Bravo Login"}},
		{"matches a severity exactly", Query{Severity: &warning}, []string{"Bravo Login"}},
		{"sorts by severity as a level rather than a word", Query{Sort: []Sort{{Field: SortBySeverity, Descending: true}}}, []string{"Charlie Error", "Bravo Login", "Alpha Scan"}},
		{"sorts by name ascending", Query{Sort: []Sort{{Field: SortByName}}}, []string{"Alpha Scan", "Bravo Login", "Charlie Error"}},
		{"breaks a tie with the next key", Query{Sort: []Sort{{Field: SortByKind}, {Field: SortByCreated, Descending: true}}}, []string{"Bravo Login", "Charlie Error", "Alpha Scan"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.query.MinDate = &fixture.future
			got, total := fixture.names(t, test.query)
			if !slices.Equal(got, test.want) {
				t.Errorf("entries = %v, want %v", got, test.want)
			}
			if total != len(test.want) {
				t.Errorf("total = %d, want %d", total, len(test.want))
			}
		})
	}
}
