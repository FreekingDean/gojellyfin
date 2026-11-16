package items

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_RecentlyPlayed(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	user := fixture.viewer(t)

	older := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Older", sortName: "older"})
	newer := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Newer", sortName: "newer"})
	started := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Started", sortName: "started"})
	show := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Show", sortName: "show"})

	now := time.Now()
	for _, datum := range []*Datum{
		{UserID: user, ItemID: older, Played: true, LastPlayedAt: ptrTo(now.Add(-2 * time.Hour))},
		{UserID: user, ItemID: newer, Played: true, LastPlayedAt: ptrTo(now.Add(-time.Hour))},
		{UserID: user, ItemID: started, PlaybackPositionTicks: 1, LastPlayedAt: ptrTo(now)},
		{UserID: user, ItemID: show, Played: true, LastPlayedAt: ptrTo(now)},
	} {
		if err := fixture.service.SaveUserItemDatum(ctx, datum); err != nil {
			t.Fatalf("failed to save user data: %v", err)
		}
	}

	viewer := Viewer{Libraries: []uuid.UUID{fixture.libraryID}}
	played, err := fixture.service.RecentlyPlayed(ctx, viewer, user, itemmodel.KindMovie, 5)
	if err != nil {
		t.Fatalf("RecentlyPlayed: %v", err)
	}
	if len(played) != 2 || played[0].ID != newer || played[1].ID != older {
		t.Errorf("played = %v, want the two finished movies, newest first", played)
	}

	limited, err := fixture.service.RecentlyPlayed(ctx, viewer, user, itemmodel.KindMovie, 1)
	if err != nil || len(limited) != 1 {
		t.Errorf("a limit of one answered %d, %v", len(limited), err)
	}

	hidden, err := fixture.service.RecentlyPlayed(ctx, Viewer{}, user, itemmodel.KindMovie, 5)
	if err != nil || len(hidden) != 0 {
		t.Errorf("a viewer who sees nothing answered %d, %v", len(hidden), err)
	}
}

func ptrTo[T any](value T) *T {
	return &value
}
