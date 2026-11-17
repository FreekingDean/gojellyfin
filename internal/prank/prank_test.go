package prank

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type catalogue map[uuid.UUID]*items.Item

func (c catalogue) ItemByID(_ context.Context, _ items.Viewer, id uuid.UUID) (*items.Item, error) {
	item, ok := c[id]
	if !ok {
		return nil, &store.NotFoundError{}
	}

	return item, nil
}

type settings struct{ stored json.RawMessage }

func (s *settings) Configuration(context.Context, string) (json.RawMessage, error) {
	return s.stored, nil
}

func (s *settings) SetConfiguration(_ context.Context, _ string, value json.RawMessage) error {
	s.stored = value

	return nil
}

type world struct {
	service *Service
	standIn *items.Item
	episode *items.Item
	song    *items.Item
	viewer  uuid.UUID
	other   uuid.UUID
	admin   uuid.UUID
}

var now = time.Date(2027, time.April, 1, 12, 0, 0, 0, time.UTC)

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{
		standIn: &items.Item{ID: uuid.New(), Kind: itemmodel.KindMovie},
		episode: &items.Item{ID: uuid.New(), Kind: itemmodel.KindEpisode},
		song:    &items.Item{ID: uuid.New(), Kind: itemmodel.KindAudio},
		viewer:  uuid.New(),
		other:   uuid.New(),
		admin:   uuid.New(),
	}
	shelf := catalogue{w.standIn.ID: w.standIn, w.episode.ID: w.episode, w.song.ID: w.song}
	w.service = newService(shelf, &settings{}, func() time.Time { return now })

	return w
}

func (w *world) save(t *testing.T, override Override) {
	t.Helper()
	if err := w.service.Save(context.Background(), override); err != nil {
		t.Fatalf("save: %v", err)
	}
}

func at(hours int) *time.Time {
	instant := now.Add(time.Duration(hours) * time.Hour)

	return &instant
}

func TestService_Swap(t *testing.T) {
	cases := []struct {
		name     string
		override func(w *world) Override
		user     func(w *world) uuid.UUID
		item     func(w *world) *items.Item
		swapped  bool
	}{
		{
			name: "an episode becomes the stand-in inside the window",
			override: func(w *world) Override {
				return Override{Enabled: true, ItemID: &w.standIn.ID, From: at(-1), Until: at(1)}
			},
			swapped: true,
		},
		{
			name:     "nothing is swapped while it is off",
			override: func(w *world) Override { return Override{ItemID: &w.standIn.ID} },
		},
		{
			name: "nothing is swapped before the window opens",
			override: func(w *world) Override {
				return Override{Enabled: true, ItemID: &w.standIn.ID, From: at(1), Until: at(2)}
			},
		},
		{
			name: "nothing is swapped once the window closes",
			override: func(w *world) Override {
				return Override{Enabled: true, ItemID: &w.standIn.ID, From: at(-2), Until: at(-1)}
			},
		},
		{
			name:     "no window means for as long as it is on",
			override: func(w *world) Override { return Override{Enabled: true, ItemID: &w.standIn.ID} },
			swapped:  true,
		},
		{
			name:     "an administrator is pranked like anyone else",
			override: func(w *world) Override { return Override{Enabled: true, ItemID: &w.standIn.ID} },
			user:     func(w *world) uuid.UUID { return w.admin },
			swapped:  true,
		},
		{
			name: "an administrator can be the only one on the list",
			override: func(w *world) Override {
				return Override{Enabled: true, ItemID: &w.standIn.ID, UserIDs: []uuid.UUID{w.admin}}
			},
			user:    func(w *world) uuid.UUID { return w.admin },
			swapped: true,
		},
		{
			name: "a user outside the chosen list is left alone",
			override: func(w *world) Override {
				return Override{Enabled: true, ItemID: &w.standIn.ID, UserIDs: []uuid.UUID{w.other}}
			},
		},
		{
			name: "a user on the chosen list is pranked",
			override: func(w *world) Override {
				return Override{Enabled: true, ItemID: &w.standIn.ID, UserIDs: []uuid.UUID{w.viewer}}
			},
			swapped: true,
		},
		{
			name:     "music plays as it is",
			override: func(w *world) Override { return Override{Enabled: true, ItemID: &w.standIn.ID} },
			item:     func(w *world) *items.Item { return w.song },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			w.save(t, tc.override(w))
			user, item := w.viewer, w.episode
			if tc.user != nil {
				user = tc.user(w)
			}
			if tc.item != nil {
				item = tc.item(w)
			}

			played, swapped, err := w.service.Swap(context.Background(), user, item)
			if err != nil {
				t.Fatalf("swap: %v", err)
			}
			if swapped != tc.swapped {
				t.Fatalf("swapped = %v, want %v", swapped, tc.swapped)
			}
			want := item
			if tc.swapped {
				want = w.standIn
			}
			if played.ID != want.ID {
				t.Fatalf("played %s, want %s", played.ID, want.ID)
			}

			disguised, err := w.service.Disguises(context.Background(), user, item.ID)
			if err != nil {
				t.Fatalf("disguises: %v", err)
			}
			if disguised != tc.swapped {
				t.Fatalf("disguises = %v, want %v", disguised, tc.swapped)
			}
		})
	}
}

func TestService_Save(t *testing.T) {
	w := newWorld(t)
	missing := uuid.New()
	cases := []struct {
		name     string
		override Override
	}{
		{name: "on with nothing to play", override: Override{Enabled: true}},
		{name: "a window that ends before it starts", override: Override{ItemID: &w.standIn.ID, From: at(1), Until: at(-1)}},
		{name: "a title nobody holds", override: Override{ItemID: &missing}},
		{name: "a song", override: Override{ItemID: &w.song.ID}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := w.service.Save(context.Background(), tc.override); !errors.Is(err, ErrInvalid) {
				t.Fatalf("save = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestService_OverrideDefaultsToAprilFirst(t *testing.T) {
	w := newWorld(t)
	w.service.now = func() time.Time { return time.Date(2026, time.November, 16, 0, 0, 0, 0, time.UTC) }

	override, err := w.service.Override(context.Background())
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if override.Enabled {
		t.Fatal("a fresh install has the override on")
	}
	want := time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC)
	if !override.From.Equal(want) || !override.Until.Equal(want.AddDate(0, 0, 1)) {
		t.Fatalf("window = %s to %s, want April 1 2027", override.From, override.Until)
	}
}
