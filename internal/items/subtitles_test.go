package items

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

func (f *fixture) probed(t *testing.T, streams ...Stream) (*Item, *MediaSource) {
	t.Helper()

	item := f.item(t, f.add(t, seed{kind: itemmodal.KindMovie, name: "Blade Runner"}))
	source := f.source(t, item.ID, "/media/Blade Runner.mkv")
	err := f.service.SaveProbe(context.Background(), item, source, Probe{
		Container: "mkv",
		Streams:   streams,
	})
	if err != nil {
		t.Fatalf("failed to save the probe: %v", err)
	}

	return item, source
}

func (f *fixture) item(t *testing.T, id uuid.UUID) *Item {
	t.Helper()

	item, err := f.service.ItemByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to load the item: %v", err)
	}

	return item
}

func (f *fixture) subtitles(t *testing.T, itemID uuid.UUID) []*MediaStream {
	t.Helper()

	streams, err := f.service.store.MediaStream.Query().
		Where(
			streammodal.HasSourceWith(sourcemodal.ItemID(itemID)),
			streammodal.KindEQ(streammodal.KindSubtitle),
		).
		Order(streammodal.ByIndex()).
		All(context.Background())
	if err != nil {
		t.Fatalf("failed to query the subtitles: %v", err)
	}

	return streams
}

func (f *fixture) warmConnection(t *testing.T) {
	t.Helper()

	if _, err := f.service.store.Item.Query().Count(context.Background()); err != nil {
		t.Errorf("failed to warm a connection: %v", err)
	}
}

func indices(streams []*MediaStream) []int32 {
	found := make([]int32, 0, len(streams))
	for _, stream := range streams {
		found = append(found, stream.Index)
	}

	return found
}

func TestService_ReplaceExternalSubtitles(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	item, source := fixture.probed(t,
		Stream{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
		Stream{Index: 1, Kind: streammodal.KindAudio, Codec: "aac"},
	)

	english := ExternalSubtitle{Path: "/media/Blade Runner.en.srt", Language: "en", Codec: "srt"}
	french := ExternalSubtitle{Path: "/media/Blade Runner.fr.srt", Language: "fr", Codec: "srt", IsForced: true}

	t.Run("indexes the external tracks after the container's own", func(t *testing.T) {
		if err := fixture.service.ReplaceExternalSubtitles(ctx, item.ID, source, []ExternalSubtitle{english, french}); err != nil {
			t.Fatalf("failed to replace the subtitles: %v", err)
		}

		found := fixture.subtitles(t, item.ID)
		if got := indices(found); len(got) != 2 || got[0] != 2 || got[1] != 3 {
			t.Fatalf("indices = %v, want [2 3]", got)
		}
		if !found[0].IsExternal || found[0].Language != "en" {
			t.Errorf("first track = %+v, want an external en track", found[0])
		}
		if !found[1].IsForced {
			t.Error("the fr track should be forced")
		}
		if !fixture.item(t, item.ID).HasSubtitles {
			t.Error("the item should have subtitles")
		}
	})

	t.Run("keeps one row per file when it runs again", func(t *testing.T) {
		if err := fixture.service.ReplaceExternalSubtitles(ctx, item.ID, source, []ExternalSubtitle{english, french}); err != nil {
			t.Fatalf("failed to replace the subtitles: %v", err)
		}

		if got := indices(fixture.subtitles(t, item.ID)); len(got) != 2 || got[0] != 2 || got[1] != 3 {
			t.Fatalf("indices = %v, want [2 3]", got)
		}
	})

	t.Run("drops a track that left the disk", func(t *testing.T) {
		if err := fixture.service.ReplaceExternalSubtitles(ctx, item.ID, source, []ExternalSubtitle{english}); err != nil {
			t.Fatalf("failed to replace the subtitles: %v", err)
		}

		found := fixture.subtitles(t, item.ID)
		if len(found) != 1 || found[0].Language != "en" {
			t.Fatalf("subtitles = %v, want the en track alone", indices(found))
		}
	})

	t.Run("clears the flag when the last track goes", func(t *testing.T) {
		if err := fixture.service.ReplaceExternalSubtitles(ctx, item.ID, source, nil); err != nil {
			t.Fatalf("failed to replace the subtitles: %v", err)
		}

		if found := fixture.subtitles(t, item.ID); len(found) != 0 {
			t.Errorf("subtitles = %v, want none", indices(found))
		}
		if fixture.item(t, item.ID).HasSubtitles {
			t.Error("the item should no longer have subtitles")
		}
	})
	t.Run("indexes from zero without a probed source", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		item := fixture.item(t, fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Unprobed"}))
		source := fixture.source(t, item.ID, "/media/Unprobed.mkv")
		subtitle := ExternalSubtitle{Path: "/media/Unprobed.en.srt", Language: "en", Codec: "srt"}

		if err := fixture.service.ReplaceExternalSubtitles(ctx, item.ID, source, []ExternalSubtitle{subtitle}); err != nil {
			t.Fatalf("failed to replace the subtitles: %v", err)
		}

		found := fixture.subtitles(t, item.ID)
		if len(found) != 1 || found[0].Index != 0 {
			t.Fatalf("indices = %v, want [0]", indices(found))
		}
		if !fixture.item(t, item.ID).HasSubtitles {
			t.Error("the item should have subtitles")
		}
	})

	t.Run("keeps the flag for embedded tracks", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		item, source := fixture.probed(t,
			Stream{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
			Stream{Index: 1, Kind: streammodal.KindSubtitle, Codec: "subrip"},
		)

		if err := fixture.service.ReplaceExternalSubtitles(ctx, item.ID, source, nil); err != nil {
			t.Fatalf("failed to replace the subtitles: %v", err)
		}

		if got := indices(fixture.subtitles(t, item.ID)); len(got) != 1 || got[0] != 1 {
			t.Fatalf("indices = %v, want the embedded track alone", got)
		}
		if !fixture.item(t, item.ID).HasSubtitles {
			t.Error("the embedded track should keep the flag set")
		}
	})

	t.Run("two runs at once still leave one row per file", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		item, source := fixture.probed(t, Stream{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"})
		subtitles := []ExternalSubtitle{
			{Path: "/media/Blade Runner.en.srt", Language: "en", Codec: "srt"},
			{Path: "/media/Blade Runner.fr.srt", Language: "fr", Codec: "srt"},
		}

		var connected, finished sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, 2)
		for i := range errs {
			connected.Add(1)
			finished.Add(1)
			go func() {
				defer finished.Done()
				fixture.warmConnection(t)
				connected.Done()
				<-start
				errs[i] = fixture.service.ReplaceExternalSubtitles(ctx, item.ID, source, subtitles)
			}()
		}
		connected.Wait()
		close(start)
		finished.Wait()

		for _, err := range errs {
			if err != nil {
				t.Fatalf("failed to replace the subtitles: %v", err)
			}
		}

		if got := indices(fixture.subtitles(t, item.ID)); len(got) != 2 || got[0] != 1 || got[1] != 2 {
			t.Errorf("indices = %v, want [1 2]", got)
		}
	})

}

func TestService_SubtitleStream(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	item, source := fixture.probed(t, Stream{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"})
	subtitle := ExternalSubtitle{Path: "/media/Blade Runner.en.srt", Language: "en", Codec: "srt"}
	if err := fixture.service.ReplaceExternalSubtitles(ctx, item.ID, source, []ExternalSubtitle{subtitle}); err != nil {
		t.Fatalf("failed to replace the subtitles: %v", err)
	}

	stream, err := fixture.service.SubtitleStream(ctx, item.ID, 1)
	if err != nil {
		t.Fatalf("failed to query the stream: %v", err)
	}
	if stream == nil || stream.Path != subtitle.Path {
		t.Fatalf("stream = %+v, want the en track", stream)
	}

	t.Run("misses an index that is not a subtitle", func(t *testing.T) {
		stream, err := fixture.service.SubtitleStream(ctx, item.ID, 0)
		if err != nil {
			t.Fatalf("failed to query the stream: %v", err)
		}
		if stream != nil {
			t.Errorf("stream = %+v, want nil for the video track", stream)
		}
	})
}
