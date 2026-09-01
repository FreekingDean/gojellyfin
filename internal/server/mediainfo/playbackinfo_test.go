package mediainfo

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

type fixture struct {
	server  *Server
	library uuid.UUID
}

func newFixture(t *testing.T) *fixture {
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

	ctx := context.Background()
	client := connection.Client()
	libraryService := libraries.New(client)

	library, err := libraryService.CreateLibrary(ctx, t.Name()+"-"+uuid.NewString(), librarymodal.CollectionTypeMovies, nil)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		inLibrary := sourcemodal.HasItemWith(itemmodal.LibraryID(library.ID))
		if _, err := client.MediaStream.Delete().Where(streammodal.HasSourceWith(inLibrary)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media streams: %v", err)
		}
		if _, err := client.MediaSource.Delete().Where(inLibrary).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if err := libraryService.DeleteLibrary(ctx, library.ID); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{server: New(items.New(client)), library: library.ID}
}

func (f *fixture) addRip(t *testing.T, kind items.Kind, container, codec string) uuid.UUID {
	t.Helper()

	return f.addRipped(t, kind, container, "h264", codec)
}

func (f *fixture) addRipped(t *testing.T, kind items.Kind, container, video, codec string) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	service := f.server.items

	item, err := service.SaveScanned(ctx, items.Scanned{
		LibraryID:    f.library,
		Kind:         kind,
		Key:          string(kind) + ":" + container + ":" + video + ":" + codec,
		Name:         "rip." + container,
		SortName:     "rip." + container,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	source, err := service.SaveSource(ctx, items.ScannedSource{
		LibraryID:    f.library,
		ItemID:       item.ID,
		Path:         "/media/rip." + container,
		Name:         "rip." + container,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = service.SaveProbe(ctx, item, source, items.Probe{
		Container: container,
		Streams: []items.Stream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: video, Height: 1080, Width: 1920},
			{Index: 1, Kind: streammodal.KindAudio, Codec: codec},
		},
	})
	if err != nil {
		t.Fatalf("failed to probe the source: %v", err)
	}

	return item.ID
}

func (f *fixture) addCopy(t *testing.T, id uuid.UUID, path, video, audio string, height int32) {
	t.Helper()

	ctx := context.Background()
	service := f.server.items

	item, err := service.ItemByID(ctx, id)
	if err != nil {
		t.Fatalf("failed to read the item: %v", err)
	}

	source, err := service.SaveSource(ctx, items.ScannedSource{
		LibraryID:    f.library,
		ItemID:       id,
		Path:         path,
		Name:         path,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = service.SaveProbe(ctx, item, source, items.Probe{
		Container: strings.TrimPrefix(filepath.Ext(path), "."),
		Streams: []items.Stream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: video, Height: height, Width: height * 16 / 9},
			{Index: 1, Kind: streammodal.KindAudio, Codec: audio},
		},
	})
	if err != nil {
		t.Fatalf("failed to probe the source: %v", err)
	}
}

func (f *fixture) unscanned(t *testing.T) uuid.UUID {
	t.Helper()

	item, err := f.server.items.SaveScanned(context.Background(), items.Scanned{
		LibraryID:    f.library,
		Kind:         itemmodal.KindMovie,
		Key:          "movie:unscanned",
		Name:         "unscanned",
		SortName:     "unscanned",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	return item.ID
}

func firstPlay(profile *api.DeviceProfile) *api.PlaybackInfoDto {
	return &api.PlaybackInfoDto{
		AutoOpenLiveStream:  apiutil.Ptr(true),
		StartTimeTicks:      apiutil.Ptr(int64(0)),
		MaxStreamingBitrate: apiutil.Ptr(int32(140000000)),
		DeviceProfile:       profile,
	}
}

func afterPlaybackError(profile *api.DeviceProfile) *api.PlaybackInfoDto {
	return &api.PlaybackInfoDto{
		AutoOpenLiveStream:   apiutil.Ptr(true),
		StartTimeTicks:       apiutil.Ptr(int64(0)),
		EnableDirectPlay:     apiutil.Ptr(false),
		EnableDirectStream:   apiutil.Ptr(false),
		AllowVideoStreamCopy: apiutil.Ptr(false),
		DeviceProfile:        profile,
	}
}

func (f *fixture) sourceID(t *testing.T, id uuid.UUID, path string) string {
	t.Helper()

	sources, err := f.server.items.MediaSources(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to read the sources: %v", err)
	}
	for _, source := range sources {
		if source.Path == path {
			return source.ID.String()
		}
	}
	t.Fatalf("the fixture has no source at %q", path)

	return ""
}

func (f *fixture) answer(t *testing.T, id uuid.UUID, body *api.PlaybackInfoDto) api.PlaybackInfoResponse {
	t.Helper()

	return f.answered(t, api.GetPostedPlaybackInfoRequestObject{ItemId: id, JSONBody: body})
}

func (f *fixture) answered(t *testing.T, request api.GetPostedPlaybackInfoRequestObject) api.PlaybackInfoResponse {
	t.Helper()

	ctx := auth.ContextWithAuthorization(context.Background(), auth.Authorization{Token: "the-token"})
	response, err := f.server.GetPostedPlaybackInfo(ctx, request)
	if err != nil {
		t.Fatalf("failed to answer playback info: %v", err)
	}

	answered, ok := response.(api.GetPostedPlaybackInfo200JSONResponse)
	if !ok {
		t.Fatalf("playback info answered %T, want a 200", response)
	}

	return api.PlaybackInfoResponse(answered)
}

func (f *fixture) sources(t *testing.T, id uuid.UUID, body *api.PlaybackInfoDto) []api.MediaSourceInfo {
	t.Helper()

	return *f.answer(t, id, body).MediaSources
}

func (f *fixture) source(t *testing.T, id uuid.UUID, body *api.PlaybackInfoDto) api.MediaSourceInfo {
	t.Helper()

	sources := f.sources(t, id, body)
	if len(sources) != 1 {
		t.Fatalf("got %d media sources, want 1", len(sources))
	}

	return sources[0]
}

func TestServer_GetPostedPlaybackInfo(t *testing.T) {
	chrome := profileFrom(t, chromeProfile)

	t.Run("the client is left with one url and no choice of path", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3")

		source := fixture.source(t, id, firstPlay(&chrome))

		if apiutil.Deref(source.SupportsDirectPlay) || apiutil.Deref(source.SupportsDirectStream) {
			t.Error("the client was offered a second path to the same item")
		}
		if !apiutil.Deref(source.SupportsTranscoding) || source.TranscodingUrl == nil {
			t.Fatal("the client was left with nothing to play")
		}
		if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHttp {
			t.Errorf("transcoding protocol = %q, want http", got)
		}
	})

	t.Run("an item with several files answers with one of them", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "aac")
		fixture.addCopy(t, id, "/media/second.mkv", "h264", "aac", 1080)

		if got := len(fixture.sources(t, id, firstPlay(&chrome))); got != 1 {
			t.Errorf("answered with %d sources, want the one the client should play", got)
		}
	})

	t.Run("the version the client names is not the client's to choose", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "aac")
		fixture.addCopy(t, id, "/media/uhd.mkv", "h264", "aac", 2160)
		named := fixture.sourceID(t, id, "/media/rip.mkv")

		body := firstPlay(&chrome)
		body.MediaSourceId = apiutil.Ptr(named)

		answer := fixture.answered(t, api.GetPostedPlaybackInfoRequestObject{
			ItemId:   id,
			Params:   api.GetPostedPlaybackInfoParams{MediaSourceId: apiutil.Ptr(named)},
			JSONBody: body,
		})

		sources := *answer.MediaSources
		if len(sources) != 1 {
			t.Fatalf("got %d media sources, want 1", len(sources))
		}
		if got := apiutil.Deref(sources[0].Path); got != "/media/uhd.mkv" {
			t.Errorf("answered the file at %q, want the one this server chose", got)
		}
	})

	t.Run("nothing is answered as a stream that has to be opened, looped or probed", func(t *testing.T) {
		fixture := newFixture(t)
		for _, id := range []uuid.UUID{
			fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3"),
			fixture.addRip(t, itemmodal.KindAudio, "flac", "flac"),
		} {
			source := fixture.source(t, id, firstPlay(&chrome))

			for name, claimed := range map[string]*bool{
				"IsRemote":         source.IsRemote,
				"IsInfiniteStream": source.IsInfiniteStream,
				"RequiresOpening":  source.RequiresOpening,
				"RequiresClosing":  source.RequiresClosing,
				"RequiresLooping":  source.RequiresLooping,
				"SupportsProbing":  source.SupportsProbing,
			} {
				if apiutil.Deref(claimed) {
					t.Errorf("%s is true for a file on this server's disk, which nothing here determined", name)
				}
			}
		}
	})

	t.Run("a song is answered with a way to reach its bytes", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindAudio, "flac", "flac")

		source := fixture.source(t, id, firstPlay(&chrome))

		if !apiutil.Deref(source.SupportsDirectPlay) && !apiutil.Deref(source.SupportsDirectStream) {
			t.Error("a song was answered with no way to reach it, which jellyfin-web reports as no compatible stream")
		}
	})

	t.Run("audio the client cannot decode is answered in a container it can", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3")

		source := fixture.source(t, id, firstPlay(&chrome))

		if got := apiutil.Deref(source.TranscodingContainer); got != "mp4" {
			t.Errorf("transcoding container = %q, want mp4", got)
		}
		url := apiutil.Deref(source.TranscodingUrl)
		if !strings.HasPrefix(url, "/Videos/"+id.String()+"/stream.mp4?") {
			t.Errorf("transcoding url = %q, want a stream.mp4 under the item", url)
		}
		if !strings.Contains(url, "container=mp4") {
			t.Errorf("transcoding url = %q, want it to name the container it will carry", url)
		}
		if !strings.Contains(url, "api_key=the-token") {
			t.Errorf("transcoding url carries no token: %q", url)
		}
	})

	t.Run("a source the client declared is named as the source", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "aac")

		source := fixture.source(t, id, firstPlay(&chrome))

		if got := apiutil.Deref(source.TranscodingContainer); got != "mkv" {
			t.Errorf("transcoding container = %q, want mkv", got)
		}
		url := apiutil.Deref(source.TranscodingUrl)
		if !strings.HasPrefix(url, "/Videos/"+id.String()+"/stream.mkv?") {
			t.Errorf("transcoding url = %q, want a stream.mkv under the item", url)
		}
		if !strings.Contains(url, "audioCodec=aac") {
			t.Errorf("transcoding url = %q, want it to name the audio it will carry", url)
		}
	})

	t.Run("codecs declared and the container not is answered by a mux", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "avi", "aac")

		source := fixture.source(t, id, firstPlay(&chrome))

		if got := apiutil.Deref(source.TranscodingContainer); got != "mp4" {
			t.Errorf("transcoding container = %q, want mp4", got)
		}
		if got := apiutil.Deref(source.TranscodingUrl); !strings.Contains(got, "audioCodec=aac") {
			t.Errorf("transcoding url = %q, want the source audio kept", got)
		}
	})

	t.Run("a client that declared nothing is handed the source", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3")

		source := fixture.source(t, id, firstPlay(nil))

		if got := apiutil.Deref(source.TranscodingContainer); got != "mkv" {
			t.Errorf("transcoding container = %q, want the source left alone", got)
		}
		if got := apiutil.Deref(source.TranscodingUrl); !strings.Contains(got, "audioCodec=ac3") {
			t.Errorf("transcoding url = %q, want the source audio named", got)
		}
	})

	t.Run("a seek is asked for by position", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3")

		body := firstPlay(&chrome)
		body.StartTimeTicks = apiutil.Ptr(int64(25_000_000))

		if got := apiutil.Deref(fixture.source(t, id, body).TranscodingUrl); !strings.Contains(got, "startTimeTicks=25000000") {
			t.Errorf("transcoding url = %q, want the position the client seeked to", got)
		}
	})

	t.Run("a picture nothing declared can carry is answered as no compatible stream", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRipped(t, itemmodal.KindMovie, "mkv", "mpeg4", "aac")

		answer := fixture.answer(t, id, firstPlay(&chrome))

		if got := len(*answer.MediaSources); got != 0 {
			t.Errorf("answered with %d sources, want none the client cannot play", got)
		}
		if got := apiutil.Deref(answer.ErrorCode); got != api.NoCompatibleStream {
			t.Errorf("error code = %q, want %q", got, api.NoCompatibleStream)
		}
	})

	t.Run("an item with no file behind it is answered as no compatible stream", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.unscanned(t)

		answer := fixture.answer(t, id, firstPlay(&chrome))

		if got := len(*answer.MediaSources); got != 0 {
			t.Errorf("answered with %d sources, want none invented for a file that is not there", got)
		}
		if got := apiutil.Deref(answer.ErrorCode); got != api.NoCompatibleStream {
			t.Errorf("error code = %q, want %q", got, api.NoCompatibleStream)
		}
	})

	t.Run("a playable source is not refused for one it has beside it", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3")

		if fixture.answer(t, id, firstPlay(&chrome)).ErrorCode != nil {
			t.Error("a source that can be muxed was answered as no compatible stream")
		}
	})

	t.Run("a song is left to the audio path", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindAudio, "flac", "flac")

		source := fixture.source(t, id, firstPlay(&chrome))

		if source.TranscodingUrl != nil {
			t.Errorf("a song was sent for a video remux: %q", *source.TranscodingUrl)
		}
	})

	t.Run("transcoding and a url to transcode from always travel together", func(t *testing.T) {
		fixture := newFixture(t)
		ids := []uuid.UUID{
			fixture.addRipped(t, itemmodal.KindMovie, "mkv", "mpeg4", "aac"),
			fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3"),
			fixture.addRip(t, itemmodal.KindMovie, "mkv", "aac"),
			fixture.addRip(t, itemmodal.KindAudio, "flac", "flac"),
			fixture.unscanned(t),
		}

		for _, body := range []*api.PlaybackInfoDto{
			firstPlay(&chrome), firstPlay(nil), afterPlaybackError(&chrome), afterPlaybackError(nil),
		} {
			for _, id := range ids {
				for _, source := range fixture.sources(t, id, body) {
					claimed := apiutil.Deref(source.SupportsTranscoding)
					if claimed && source.TranscodingUrl == nil {
						t.Errorf("source %q claims transcoding with no url, which jellyfin-web retries forever", apiutil.Deref(source.Name))
					}
					if !claimed && source.TranscodingUrl != nil {
						t.Errorf("source %q hands over a url the client will never read", apiutil.Deref(source.Name))
					}
				}
			}
		}
	})

	t.Run("a playback error has somewhere to stop", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3")

		raw := strings.ToLower(apiutil.Deref(fixture.source(t, id, afterPlaybackError(&chrome)).TranscodingUrl))

		for _, refusal := range []string{"allowvideostreamcopy=false", "allowaudiostreamcopy=false"} {
			if !strings.Contains(raw, refusal) {
				t.Errorf("url = %q, want it to carry %q so jellyfin-web stops retrying", raw, refusal)
			}
		}
	})
}
