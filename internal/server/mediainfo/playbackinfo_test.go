package mediainfo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

type showFixture struct {
	server    *Server
	service   *items.Service
	libraryID uuid.UUID
}

func newShowFixture(t *testing.T) *showFixture {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := items.New(client)

	return &showFixture{server: New(service), service: service, libraryID: library.ID}
}

func (f *showFixture) folder(t *testing.T, kind items.Kind, name string, parent *uuid.UUID) uuid.UUID {
	t.Helper()

	record, err := f.service.SaveScanned(context.Background(), items.Scanned{
		LibraryID:    f.libraryID,
		ParentID:     parent,
		Kind:         kind,
		Key:          fmt.Sprintf("test:%s:%s", kind, name),
		Name:         name,
		SortName:     name,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save %q: %v", name, err)
	}

	return record.ID
}

func (f *showFixture) episode(t *testing.T, season uuid.UUID, name, path string, probed bool) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	record, err := f.service.SaveScanned(ctx, items.Scanned{
		LibraryID:    f.libraryID,
		ParentID:     &season,
		Kind:         itemmodal.KindEpisode,
		Key:          fmt.Sprintf("test:episode:%s", name),
		Name:         name,
		SortName:     name,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save %q: %v", name, err)
	}
	if !probed {
		return record.ID
	}

	source, err := f.service.SaveSource(ctx, items.ScannedSource{
		LibraryID: f.libraryID,
		ItemID:    record.ID,
		Path:      path,
		Name:      name,
	})
	if err != nil {
		t.Fatalf("failed to save the source of %q: %v", name, err)
	}

	probe := items.Probe{
		Container:    "mkv",
		RunTimeTicks: 12000000000,
		Size:         5000000,
		Bitrate:      4000000,
		Streams: []items.Stream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: "h264", Width: 1920, Height: 1080, IsDefault: true},
			{Index: 1, Kind: streammodal.KindAudio, Codec: "ac3", Channels: 6, SampleRate: 48000, IsDefault: true},
		},
	}
	if err := f.service.SaveProbe(ctx, record, source, probe); err != nil {
		t.Fatalf("failed to probe %q: %v", name, err)
	}

	return record.ID
}

func firstPlay() *api.PlaybackInfoDto {
	return &api.PlaybackInfoDto{
		AutoOpenLiveStream:  apiutil.Ptr(true),
		StartTimeTicks:      apiutil.Ptr(int64(0)),
		MaxStreamingBitrate: apiutil.Ptr(int32(140000000)),
	}
}

func afterPlaybackError() *api.PlaybackInfoDto {
	return &api.PlaybackInfoDto{
		AutoOpenLiveStream:   apiutil.Ptr(true),
		StartTimeTicks:       apiutil.Ptr(int64(0)),
		EnableDirectPlay:     apiutil.Ptr(false),
		EnableDirectStream:   apiutil.Ptr(false),
		AllowVideoStreamCopy: apiutil.Ptr(false),
	}
}

func (f *showFixture) playbackInfo(t *testing.T, itemID uuid.UUID, body *api.PlaybackInfoDto) api.PlaybackInfoResponse {
	t.Helper()

	response, err := f.server.GetPostedPlaybackInfo(context.Background(), api.GetPostedPlaybackInfoRequestObject{
		ItemId:   itemID,
		JSONBody: body,
	})
	if err != nil {
		t.Fatalf("failed to answer PlaybackInfo: %v", err)
	}

	answered, ok := response.(api.GetPostedPlaybackInfo200JSONResponse)
	if !ok {
		t.Fatalf("PlaybackInfo answered %T, want a 200", response)
	}

	return api.PlaybackInfoResponse(answered)
}

func TestServer_GetPostedPlaybackInfo(t *testing.T) {
	fixture := newShowFixture(t)

	series := fixture.folder(t, itemmodal.KindSeries, "Fixture Show", nil)
	season := fixture.folder(t, itemmodal.KindSeason, "Fixture Show Season 1", &series)
	episode := fixture.episode(t, season, "Fixture Show S01E01", "/media/shows/fixture/s01e01.mkv", true)
	unprobed := fixture.episode(t, season, "Fixture Show S01E02", "/media/shows/fixture/s01e02.mkv", false)

	t.Run("an episode answers with a source the client can identify", func(t *testing.T) {
		response := fixture.playbackInfo(t, episode, firstPlay())

		if response.MediaSources == nil || len(*response.MediaSources) == 0 {
			t.Fatal("no media sources, so the client has nothing to play")
		}

		source := (*response.MediaSources)[0]
		id := apiutil.Deref(source.Id)
		if _, err := uuid.Parse(id); err != nil {
			t.Errorf("media source id %q does not round-trip: %v", id, err)
		}
		if apiutil.Deref(source.Container) != "mkv" {
			t.Errorf("container = %q, want mkv", apiutil.Deref(source.Container))
		}
		if source.MediaStreams == nil || len(*source.MediaStreams) != 2 {
			t.Errorf("media streams = %v, want the probed video and audio", source.MediaStreams)
		}
	})

	t.Run("a play session is minted for every answer", func(t *testing.T) {
		response := fixture.playbackInfo(t, episode, firstPlay())

		id := apiutil.Deref(response.PlaySessionId)
		if _, err := uuid.Parse(id); err != nil {
			t.Errorf("play session id %q does not round-trip: %v", id, err)
		}
	})

	t.Run("every source offers a way to play", func(t *testing.T) {
		body := firstPlay()
		for _, itemID := range []uuid.UUID{episode, unprobed} {
			for _, source := range *fixture.playbackInfo(t, itemID, body).MediaSources {
				playable := apiutil.Deref(source.SupportsDirectPlay) ||
					apiutil.Deref(source.SupportsDirectStream) ||
					apiutil.Deref(source.SupportsTranscoding)
				if !playable {
					t.Errorf("source %q offers no way to play", apiutil.Deref(source.Name))
				}
			}
		}
	})

	t.Run("transcoding is never claimed without a url to transcode from", func(t *testing.T) {
		for _, body := range []*api.PlaybackInfoDto{firstPlay(), afterPlaybackError()} {
			for _, itemID := range []uuid.UUID{episode, unprobed} {
				for _, source := range *fixture.playbackInfo(t, itemID, body).MediaSources {
					if apiutil.Deref(source.SupportsTranscoding) && source.TranscodingUrl == nil {
						t.Errorf("source %q claims transcoding with no TranscodingUrl, which jellyfin-web retries forever", apiutil.Deref(source.Name))
					}
				}
			}
		}
	})
}
