package activity

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
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
