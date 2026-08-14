package items

import (
	"context"
	"testing"

	"github.com/google/uuid"

	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

func TestDeleteItemsNotInKeysTakesDescendants(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Series"})
	season := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series, index: number(1)})
	episode := fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S01E01", parentID: &season, index: number(1), parentIndex: number(1)})

	kept := make([]string, 0, 2)
	for _, id := range []uuid.UUID{season, episode} {
		record, err := fixture.service.ItemByID(ctx, id)
		if err != nil {
			t.Fatalf("failed to load the item: %v", err)
		}
		kept = append(kept, record.Key)
	}

	if err := fixture.service.DeleteItemsNotInKeys(ctx, fixture.libraryID, kept); err != nil {
		t.Fatalf("failed to prune the items: %v", err)
	}

	records, total, err := fixture.service.QueryItems(ctx, ItemQuery{LibraryID: &fixture.libraryID})
	if err != nil {
		t.Fatalf("failed to query the items: %v", err)
	}
	if total != 0 || len(records) != 0 {
		t.Errorf("items = %v, want none", names(records))
	}
}

func TestDeleteItemsNotInKeysWithImages(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	kept := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Kept"})
	pruned := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Pruned"})

	artwork := []Artwork{{Kind: imagemodal.KindPrimary, Path: "/artwork/poster.jpg", Tag: "tag"}}
	if err := fixture.service.ReplaceImages(ctx, pruned, artwork); err != nil {
		t.Fatalf("failed to save the images: %v", err)
	}

	survivor, err := fixture.service.ItemByID(ctx, kept)
	if err != nil {
		t.Fatalf("failed to load the kept item: %v", err)
	}

	if err := fixture.service.DeleteItemsNotInKeys(ctx, fixture.libraryID, []string{survivor.Key}); err != nil {
		t.Fatalf("failed to prune the items: %v", err)
	}

	records, total, err := fixture.service.QueryItems(ctx, ItemQuery{LibraryID: &fixture.libraryID})
	if err != nil {
		t.Fatalf("failed to query the items: %v", err)
	}
	if total != 1 || len(records) != 1 || records[0].ID != kept {
		t.Errorf("items = %v, want only the kept item", names(records))
	}

	// A sweep marks the row rather than deleting it, so the foreign key never
	// fires and the artwork survives. That is deliberate: a volume that comes
	// back needs nothing re-derived. The purge is what takes them.
	images, err := fixture.service.Images(ctx, pruned)
	if err != nil {
		t.Fatalf("failed to query the images: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("images = %d, want the artwork kept for the item's return", len(images))
	}
}

func TestSaveProbeReplacesStreams(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	id := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Movie"})
	item, err := fixture.service.ItemByID(ctx, id)
	if err != nil {
		t.Fatalf("failed to load the item: %v", err)
	}

	first := Probe{
		Container:    "mkv",
		RunTimeTicks: 100,
		Streams: []Stream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
			{Index: 1, Kind: streammodal.KindAudio, Codec: "aac"},
		},
	}
	source := fixture.source(t, id, "/media/movie.mkv")
	if err := fixture.service.SaveProbe(ctx, item, source, first); err != nil {
		t.Fatalf("failed to save the first probe: %v", err)
	}

	second := Probe{
		Container:    "mkv",
		RunTimeTicks: 200,
		Streams:      []Stream{{Index: 0, Kind: streammodal.KindVideo, Codec: "hevc"}},
	}
	if err := fixture.service.SaveProbe(ctx, item, source, second); err != nil {
		t.Fatalf("failed to save the second probe: %v", err)
	}

	probed, err := fixture.service.MediaSource(ctx, id)
	if err != nil {
		t.Fatalf("failed to query the media source: %v", err)
	}
	if probed == nil {
		t.Fatal("media source = nil, want a probed source")
	}
	streams := probed.Edges.Streams
	if len(streams) != 1 {
		t.Fatalf("streams = %d, want 1", len(streams))
	}
	if codec := streams[0].Codec; codec != "hevc" {
		t.Errorf("codec = %q, want %q", codec, "hevc")
	}
}
