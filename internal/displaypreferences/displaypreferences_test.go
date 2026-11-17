package displaypreferences

import (
	"context"
	"testing"

	"github.com/google/uuid"

	displaypreferencesmodel "github.com/FreekingDean/gojellyfin/internal/store/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/storetest"
)

func TestService(t *testing.T) {
	ctx := context.Background()
	client := storetest.Client(t)
	service := New(client)

	name := t.Name() + "-" + uuid.NewString()
	user, err := client.User.Create().SetName(name).SetUsername(name).SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := client.DisplayPreferences.Delete().Where(displaypreferencesmodel.UserID(user.ID)).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the preferences: %v", err)
		}
		if err := client.User.DeleteOneID(user.ID).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
	})

	descending := displaypreferencesmodel.SortOrderDescending
	tests := []struct {
		name     string
		client   string
		settings *Settings
		want     func(*DisplayPreferences) bool
		stored   int
	}{
		{
			name:   "a miss serves the defaults and stores nothing",
			client: "web",
			want: func(got *DisplayPreferences) bool {
				return got.SortBy == displaypreferencesmodel.DefaultSortBy && got.SortOrder == displaypreferencesmodel.DefaultSortOrder
			},
		},
		{
			name:     "an update is read back",
			client:   "web",
			settings: &Settings{SortBy: ptr("DateCreated"), SortOrder: &descending, CustomPrefs: &map[string]string{"homesection0": "resume"}},
			want: func(got *DisplayPreferences) bool {
				return got.SortBy == "DateCreated" && got.SortOrder == descending && got.CustomPrefs["homesection0"] == "resume"
			},
			stored: 1,
		},
		{
			name:     "another client is a row of its own",
			client:   "tv",
			settings: &Settings{SortBy: ptr("SortName")},
			want:     func(got *DisplayPreferences) bool { return got.SortBy == "SortName" },
			stored:   2,
		},
		{
			name:     "a second update replaces what the first set",
			client:   "web",
			settings: &Settings{SortBy: ptr("PremiereDate")},
			want: func(got *DisplayPreferences) bool {
				return got.SortBy == "PremiereDate" && got.SortOrder == displaypreferencesmodel.DefaultSortOrder
			},
			stored: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.settings != nil {
				if err := service.Update(ctx, user.ID, "usersettings", test.client, *test.settings); err != nil {
					t.Fatalf("Update returned %v", err)
				}
			}

			got, err := service.Get(ctx, user.ID, "usersettings", test.client)
			if err != nil {
				t.Fatalf("Get returned %v", err)
			}
			if !test.want(got) {
				t.Errorf("preferences = %+v", got)
			}

			stored, err := client.DisplayPreferences.Query().Where(displaypreferencesmodel.UserID(user.ID)).Count(ctx)
			if err != nil {
				t.Fatalf("failed to count the rows: %v", err)
			}
			if stored != test.stored {
				t.Errorf("rows = %d, want %d", stored, test.stored)
			}
		})
	}
}

func ptr[T any](value T) *T { return &value }
