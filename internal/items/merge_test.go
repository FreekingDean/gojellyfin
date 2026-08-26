package items

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	datamodal "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
)

func TestService_Merge(t *testing.T) {
	t.Run("carries the duplicate forward", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		survivor := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Kept"})
		duplicate := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Folded"})

		watcher := fixture.user(t, "watcher")
		other := fixture.user(t, "other")

		played := time.Now().Add(-time.Hour).UTC()
		fixture.datum(t, watcher, survivor, &Datum{IsFavorite: true, PlaybackPositionTicks: 555})
		fixture.datum(t, watcher, duplicate, &Datum{Played: true, PlayCount: 7, LastPlayedAt: &played})
		fixture.datum(t, other, duplicate, &Datum{PlaybackPositionTicks: 999})

		entry := fixture.playlistEntry(t, watcher, duplicate)

		source, err := fixture.service.SaveSource(ctx, ScannedSource{
			LibraryID:    fixture.libraryID,
			ItemID:       duplicate,
			Path:         "/library/folded.mkv",
			Name:         "folded.mkv",
			DateModified: time.Now(),
		})
		if err != nil {
			t.Fatalf("failed to save the source: %v", err)
		}

		if err := fixture.service.Merge(ctx, duplicate, survivor); err != nil {
			t.Fatalf("failed to merge: %v", err)
		}

		if _, err := fixture.service.store.Item.Get(ctx, duplicate); err == nil {
			t.Error("the duplicate row outlived the merge")
		}

		merged, err := fixture.service.UserItemDatum(ctx, watcher, survivor)
		if err != nil {
			t.Fatalf("failed to read the merged user data: %v", err)
		}
		if !merged.Played || !merged.IsFavorite {
			t.Errorf("played = %v, favourite = %v, want both: one row's answer was dropped", merged.Played, merged.IsFavorite)
		}
		if merged.PlayCount != 7 {
			t.Errorf("play count = %d, want 7", merged.PlayCount)
		}
		if merged.PlaybackPositionTicks != 555 {
			t.Errorf("position = %d, want 555", merged.PlaybackPositionTicks)
		}
		if merged.LastPlayedAt == nil {
			t.Error("the merged datum lost the time the title was last played")
		}

		moved, err := fixture.service.UserItemDatum(ctx, other, survivor)
		if err != nil {
			t.Fatalf("failed to read the moved user data: %v", err)
		}
		if moved.PlaybackPositionTicks != 999 {
			t.Errorf("position = %d, want 999: a user the survivor knew nothing about lost their resume point", moved.PlaybackPositionTicks)
		}

		left, err := fixture.service.store.UserItemData.Query().Where(datamodal.ItemID(duplicate)).Count(ctx)
		if err != nil {
			t.Fatalf("failed to count the duplicate's user data: %v", err)
		}
		if left != 0 {
			t.Errorf("%d user data rows still point at the merged-away item", left)
		}

		record, err := fixture.service.store.PlaylistEntry.Get(ctx, entry)
		if err != nil {
			t.Fatalf("failed to read the playlist entry: %v", err)
		}
		if record.ItemID != survivor {
			t.Errorf("the playlist entry points at %s, want the survivor %s", record.ItemID, survivor)
		}

		kept, err := fixture.service.store.MediaSource.Query().Where(sourcemodal.ID(source.ID)).Only(ctx)
		if err != nil {
			t.Fatalf("failed to read the source: %v", err)
		}
		if kept.ItemID != survivor {
			t.Errorf("the file still hangs off %s, want the survivor %s", kept.ItemID, survivor)
		}
	})

	t.Run("reparents the duplicate's children", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		survivor := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Kept"})
		duplicate := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Folded"})
		season := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &duplicate, index: number(1)})

		if err := fixture.service.Merge(ctx, duplicate, survivor); err != nil {
			t.Fatalf("failed to merge: %v", err)
		}

		record, err := fixture.service.ItemByID(ctx, season)
		if err != nil {
			t.Fatalf("the season went with its series: %v", err)
		}
		if record.ParentID == nil || *record.ParentID != survivor {
			t.Errorf("the season hangs off %v, want the survivor %s", record.ParentID, survivor)
		}
	})
}

func (f *fixture) user(t *testing.T, name string) uuid.UUID {
	t.Helper()

	record, err := f.service.store.User.Create().
		SetName(name).
		SetUsername(name + "-" + uuid.NewString()).
		SetPasswordHash("hash").
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", name, err)
	}
	t.Cleanup(func() {
		if err := f.service.store.User.DeleteOneID(record.ID).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
	})

	return record.ID
}

func (f *fixture) datum(t *testing.T, userID, itemID uuid.UUID, datum *Datum) {
	t.Helper()

	datum.UserID = userID
	datum.ItemID = itemID
	if err := f.service.SaveUserItemDatum(context.Background(), datum); err != nil {
		t.Fatalf("failed to save the user data: %v", err)
	}
}

func (f *fixture) playlistEntry(t *testing.T, ownerID, itemID uuid.UUID) uuid.UUID {
	t.Helper()

	ctx := context.Background()

	holder := f.add(t, seed{kind: itemmodal.KindPlaylist, name: "Playlist"})
	playlist, err := f.service.store.Playlist.Create().SetItemID(holder).SetOwnerID(ownerID).SetOpenAccess(false).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the playlist: %v", err)
	}

	entry, err := f.service.store.PlaylistEntry.Create().
		SetPlaylistID(playlist.ID).
		SetItemID(itemID).
		SetSortOrder(0).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the playlist entry: %v", err)
	}

	return entry.ID
}
