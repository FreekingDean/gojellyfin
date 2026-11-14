package items

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func tmdbID() int {
	return int(uuid.New().ID())
}

func (f *fixture) refresh(t *testing.T, scanned Scanned) []jobs.Queued {
	t.Helper()

	enqueued, err := jobs.RunJob(t, f.service.RefreshItemJob(),
		jobs.With(jobs.ParamItem, scanned),
		jobs.With(jobs.ParamLibrary, f.libraryID),
		jobs.With(jobs.ParamSource, f.downloader(t)),
	)
	if err != nil {
		t.Fatalf("failed to refresh %s: %v", scanned.Key, err)
	}

	return enqueued
}

func TestService_RefreshItem(t *testing.T) {
	t.Run("asks for a probe and metadata for a title it has just written", func(t *testing.T) {
		fixture := newFixture(t)

		enqueued := fixture.refresh(t, Scanned{
			Key:      MovieKey(tmdbID()),
			Kind:     itemmodel.KindMovie,
			Name:     "The Matrix",
			SortName: "matrix",
			Files: []ScannedFile{{
				Path:         "/media/The Matrix/" + uuid.NewString() + ".mkv",
				DateModified: time.Now(),
			}},
		})

		if probes := jobs.Enqueued(t, enqueued, jobs.ProbeFile); len(probes) != 1 {
			t.Errorf("probes = %d, want the new file probed", len(probes))
		}
		if identify := jobs.Enqueued(t, enqueued, jobs.RefreshItemMetadata); len(identify) != 1 {
			t.Errorf("metadata = %d, want the unidentified title identified", len(identify))
		}
	})

	t.Run("asks for neither once the file is probed and the title identified", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		path := "/media/The Matrix/" + uuid.NewString() + ".mkv"

		scanned := Scanned{
			Key:      MovieKey(tmdbID()),
			Kind:     itemmodel.KindMovie,
			Name:     "The Matrix",
			SortName: "matrix",
			Files:    []ScannedFile{{Path: path, DateModified: time.Now()}},
		}
		fixture.refresh(t, scanned)

		item, err := fixture.service.ItemByKey(ctx, scanned.Key)
		if err != nil {
			t.Fatalf("failed to read the item back: %v", err)
		}

		sources, err := fixture.service.MediaSources(ctx, item.ID)
		if err != nil {
			t.Fatalf("failed to read the sources back: %v", err)
		}
		if err := fixture.service.SaveProbe(ctx, item, sources[0], MediaSource{Container: "mkv"}); err != nil {
			t.Fatalf("failed to probe: %v", err)
		}
		if _, err := fixture.service.UpdateMetadata(ctx, item.ID, Metadata{
			ProviderIds: &map[string]string{"Tmdb": "603"},
		}); err != nil {
			t.Fatalf("failed to identify: %v", err)
		}

		enqueued := fixture.refresh(t, scanned)

		if probes := jobs.Enqueued(t, enqueued, jobs.ProbeFile); len(probes) != 0 {
			t.Errorf("probes = %d, want the probed file left alone", len(probes))
		}
		if identify := jobs.Enqueued(t, enqueued, jobs.RefreshItemMetadata); len(identify) != 0 {
			t.Errorf("metadata = %d, want the identified title left alone", len(identify))
		}
	})
}

func TestService_RefreshItem_Music(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	artistKey, albumKey, trackKey := ArtistKey(uuid.NewString()), AlbumKey(uuid.NewString()), TrackKey(uuid.NewString())

	for _, scanned := range []Scanned{
		{
			Key: artistKey, Tagged: true, Kind: itemmodel.KindMusicArtist, Name: "The Band", SortName: "band",
			Images: []RemoteImage{{Kind: ImageKindPrimary, URL: "https://art/" + uuid.NewString() + ".jpg"}},
		},
		{Key: albumKey, ParentKey: artistKey, Tagged: true, Kind: itemmodel.KindMusicAlbum, Name: "First", SortName: "first"},
	} {
		if enqueued := fixture.refresh(t, scanned); len(jobs.Enqueued(t, enqueued, jobs.RefreshItemMetadata)) != 0 {
			t.Errorf("%s was sent to a provider that has nothing to say about music", scanned.Key)
		}
	}

	enqueued := fixture.refresh(t, Scanned{
		Key: trackKey, ParentKey: albumKey, Tagged: true, Kind: itemmodel.KindAudio, Name: "Opener", SortName: "opener",
		Files: []ScannedFile{{Path: "/media/music/" + uuid.NewString() + ".flac", DateModified: time.Now()}},
	})
	if probes := jobs.Enqueued(t, enqueued, jobs.ProbeFile); len(probes) != 1 {
		t.Errorf("probes = %d, want the new track probed", len(probes))
	}

	artist, err := fixture.service.ItemByKey(ctx, artistKey)
	if err != nil {
		t.Fatalf("failed to read the artist back: %v", err)
	}
	images, err := fixture.service.Images(ctx, artist.ID)
	if err != nil || len(images) != 1 || images[0].Tag == "" {
		t.Errorf("images = %+v (%v), want the artist's poster with a tag", images, err)
	}

	album, err := fixture.service.ItemByKey(ctx, albumKey)
	if err != nil || album.ParentID == nil || *album.ParentID != artist.ID {
		t.Fatalf("album = %+v (%v), want it under the artist", album, err)
	}
	track, err := fixture.service.ItemByKey(ctx, trackKey)
	if err != nil || track.ParentID == nil || *track.ParentID != album.ID {
		t.Fatalf("track = %+v (%v), want it under the album", track, err)
	}
	if !IsFolder(album.Kind) || !IsFolder(artist.Kind) || IsFolder(track.Kind) {
		t.Error("an artist and an album are folders and a track is not")
	}

	credited, _, err := fixture.service.QueryItems(ctx, ItemQuery{Viewer: Everyone, ArtistIDs: []uuid.UUID{artist.ID}})
	if err != nil || len(credited) != 2 {
		t.Errorf("by artist = %d items (%v), want the album and its track", len(credited), err)
	}
	onAlbum, _, err := fixture.service.QueryItems(ctx, ItemQuery{Viewer: Everyone, AlbumIDs: []uuid.UUID{album.ID}})
	if err != nil || len(onAlbum) != 1 || onAlbum[0].ID != track.ID {
		t.Errorf("by album = %+v (%v), want the track", onAlbum, err)
	}
}
