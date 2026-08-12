package activitylog

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
)

type fixture struct {
	server *Server
	client *store.Client
	userID uuid.UUID
	future time.Time
	ids    []uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
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
		server: New(activity.New(client)),
		client: client,
		userID: user.ID,
		future: time.Now().Add(time.Hour).Truncate(time.Millisecond),
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.ActivityLogEntry.Delete().Where(entrymodal.IDIn(f.ids...)).Exec(ctx); err != nil {
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
		SetKind(activity.KindLibraryScanCompleted).
		SetShortOverview("seeded").
		SetSeverity(activity.SeverityInformation).
		SetCreatedAt(at).
		SetNillableUserID(userID).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", name, err)
	}

	f.ids = append(f.ids, entry.ID)
}

func (f *fixture) get(t *testing.T, params api.GetLogEntriesParams) api.ActivityLogEntryQueryResult {
	t.Helper()

	response, err := f.server.GetLogEntries(context.Background(), api.GetLogEntriesRequestObject{Params: params})
	if err != nil {
		t.Fatalf("failed to get log entries: %v", err)
	}

	result, ok := response.(api.GetLogEntries200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want a 200", response)
	}

	return api.ActivityLogEntryQueryResult(result)
}

func names(result api.ActivityLogEntryQueryResult) []string {
	found := make([]string, 0)
	for _, entry := range apiutil.Deref(result.Items) {
		found = append(found, apiutil.Deref(entry.Name))
	}

	return found
}

func TestGetLogEntries(t *testing.T) {
	fixture := newFixture(t)

	fixture.add(t, "Oldest", fixture.future, nil)
	fixture.add(t, "Middle", fixture.future.Add(time.Minute), nil)
	fixture.add(t, "Newest", fixture.future.Add(2*time.Minute), nil)
	fixture.add(t, "By User", fixture.future.Add(3*time.Minute), &fixture.userID)

	t.Run("returns the newest entry first", func(t *testing.T) {
		result := fixture.get(t, api.GetLogEntriesParams{MinDate: &fixture.future})

		want := []string{"By User", "Newest", "Middle", "Oldest"}
		if got := names(result); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
		if total := apiutil.Deref(result.TotalRecordCount); total != 4 {
			t.Errorf("total = %d, want 4", total)
		}
	})

	t.Run("pages without changing the total", func(t *testing.T) {
		result := fixture.get(t, api.GetLogEntriesParams{
			MinDate:    &fixture.future,
			StartIndex: apiutil.Ptr(int32(1)),
			Limit:      apiutil.Ptr(int32(2)),
		})

		want := []string{"Newest", "Middle"}
		if got := names(result); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
		if total := apiutil.Deref(result.TotalRecordCount); total != 4 {
			t.Errorf("total = %d, want 4", total)
		}
		if start := apiutil.Deref(result.StartIndex); start != 1 {
			t.Errorf("start index = %d, want 1", start)
		}
	})

	t.Run("drops entries before the minimum date", func(t *testing.T) {
		minDate := fixture.future.Add(2 * time.Minute)
		result := fixture.get(t, api.GetLogEntriesParams{MinDate: &minDate})

		want := []string{"By User", "Newest"}
		if got := names(result); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
		if total := apiutil.Deref(result.TotalRecordCount); total != 2 {
			t.Errorf("total = %d, want 2", total)
		}
	})

	t.Run("filters on having a user", func(t *testing.T) {
		result := fixture.get(t, api.GetLogEntriesParams{MinDate: &fixture.future, HasUserId: apiutil.Ptr(true)})

		want := []string{"By User"}
		if got := names(result); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
		for _, entry := range apiutil.Deref(result.Items) {
			if got := apiutil.Deref(entry.UserId); got != fixture.userID {
				t.Errorf("user id = %v, want %v", got, fixture.userID)
			}
		}
	})

	t.Run("filters on not having a user", func(t *testing.T) {
		result := fixture.get(t, api.GetLogEntriesParams{MinDate: &fixture.future, HasUserId: apiutil.Ptr(false)})

		want := []string{"Newest", "Middle", "Oldest"}
		if got := names(result); !slices.Equal(got, want) {
			t.Errorf("entries = %v, want %v", got, want)
		}
	})
}
