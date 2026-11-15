package items

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_Lyrics(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	song := func(lyrics string) *Item {
		key := TrackKey(uuid.NewString())
		fixture.refresh(t, Scanned{
			Key: key, Tagged: true, Kind: itemmodel.KindAudio, Name: "Opener", SortName: "opener",
			Files: []ScannedFile{{Path: "/media/music/" + uuid.NewString() + ".flac", DateModified: time.Now()}},
		})

		item, err := fixture.service.ItemByKey(ctx, key)
		if err != nil {
			t.Fatalf("failed to read the track back: %v", err)
		}
		sources, err := fixture.service.MediaSources(ctx, item.ID)
		if err != nil {
			t.Fatalf("failed to read the file back: %v", err)
		}
		if err := fixture.service.SaveProbe(ctx, item, sources[0], MediaSource{Container: "flac", Lyrics: lyrics}); err != nil {
			t.Fatalf("failed to probe: %v", err)
		}

		return item
	}

	sung, silent := song("[00:01.00]hello"), song("")

	text, err := fixture.service.Lyrics(ctx, Everyone, sung.ID)
	if err != nil || text != "[00:01.00]hello" {
		t.Errorf("lyrics = %q (%v), want what the probe found", text, err)
	}
	if _, err := fixture.service.Lyrics(ctx, Everyone, silent.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want %v for a track with none", err, ErrNotFound)
	}

	held, err := fixture.service.FilesByItem(ctx, []uuid.UUID{sung.ID, silent.ID})
	if err != nil {
		t.Fatalf("failed to read the files: %v", err)
	}
	if !held[sung.ID].HasLyrics || held[silent.ID].HasLyrics {
		t.Errorf("held = %+v, want only the sung track to have lyrics", held)
	}
	if held[sung.ID].Path == "" {
		t.Error("selecting the columns the listing needs dropped the path")
	}
}
