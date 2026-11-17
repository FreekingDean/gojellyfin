package items

import (
	"context"
	"testing"

	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_WatchState(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	user := fixture.viewer(t)

	movie := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Movie", sortName: "movie"})
	runtime := int64(1000)
	if err := fixture.client.Item.UpdateOneID(movie).SetRunTimeTicks(runtime).Exec(ctx); err != nil {
		t.Fatalf("failed to give the movie a runtime: %v", err)
	}

	read := serviceWatchStateRead(t, fixture, ctx, user, movie)

	if err := fixture.service.RecordProgress(ctx, user, movie, 300); err != nil {
		t.Fatalf("RecordProgress: %v", err)
	}
	if datum := read(); datum.PlaybackPositionTicks != 300 || datum.LastPlayedAt == nil || datum.Played {
		t.Errorf("after progress = %+v, want a resume position and nothing played", datum)
	}

	if err := fixture.service.RecordStop(ctx, user, movie, 500); err != nil {
		t.Fatalf("RecordStop: %v", err)
	}
	if datum := read(); datum.PlaybackPositionTicks != 500 || datum.Played {
		t.Errorf("a stop halfway = %+v, want the position kept and nothing played", datum)
	}

	if err := fixture.service.RecordStop(ctx, user, movie, 950); err != nil {
		t.Fatalf("RecordStop: %v", err)
	}
	if datum := read(); datum.PlaybackPositionTicks != 0 || !datum.Played || datum.PlayCount != 1 {
		t.Errorf("a stop past the threshold = %+v, want it played once with no resume position", datum)
	}

	unplayed, err := fixture.service.MarkPlayed(ctx, user, movie, false)
	if err != nil {
		t.Fatalf("MarkPlayed: %v", err)
	}
	if unplayed.Played || unplayed.PlayCount != 1 {
		t.Errorf("unmarked = %+v, want unplayed with the count kept", unplayed)
	}

	played, err := fixture.service.MarkPlayed(ctx, user, movie, true)
	if err != nil {
		t.Fatalf("MarkPlayed: %v", err)
	}
	if !played.Played || played.PlayCount != 2 || played.PlaybackPositionTicks != 0 {
		t.Errorf("marked = %+v, want played twice with no resume position", played)
	}

	favourite, err := fixture.service.MarkFavorite(ctx, user, movie, true)
	if err != nil {
		t.Fatalf("MarkFavorite: %v", err)
	}
	if !favourite.IsFavorite || !read().IsFavorite {
		t.Errorf("favourite = %+v, want it stored", favourite)
	}
}

func serviceWatchStateRead(t *testing.T, fixture *fixture, ctx context.Context, user uuid.UUID, movie uuid.UUID) func() *Datum {
	t.Helper()

	return func() *Datum {
		t.Helper()
		datum, err := fixture.service.UserItemDatum(ctx, user, movie)
		if err != nil {
			t.Fatalf("failed to read the user data: %v", err)
		}

		return datum
	}
}
