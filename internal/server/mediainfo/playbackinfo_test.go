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
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"
	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	downloadermodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type fixture struct {
	server     *Server
	library    uuid.UUID
	downloader uuid.UUID
	client     *store.Client
	keyframes  *items.KeyframeIndex
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

	library, err := libraryService.CreateLibrary(ctx, t.Name()+"-"+uuid.NewString(), librarymodel.CollectionTypeMovies, nil)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		inLibrary := sourcemodel.HasItemWith(itemmodel.HasLibrariesWith(librarymembership.LibraryID(library.ID)))
		if _, err := client.MediaStream.Delete().Where(streammodel.HasSourceWith(inLibrary)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media streams: %v", err)
		}
		if _, err := client.ItemSource.Delete().Where(inLibrary).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if err := libraryService.DeleteLibrary(ctx, library.ID); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	downloader, err := client.Source.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(downloadermodel.KindRadarr).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Source.DeleteOne(downloader).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	return &fixture{
		downloader: downloader.ID,
		client:     client,
		server:     New(items.New(client), allLibraries{}, nil, nil),
		library:    library.ID,
		keyframes:  &items.KeyframeIndex{Ticks: []int64{0, 60_000_000}},
	}
}

func (f *fixture) addRip(t *testing.T, kind items.Kind, container, codec string) uuid.UUID {
	t.Helper()

	return f.addRipped(t, kind, container, "h264", codec)
}

func (f *fixture) addRipped(t *testing.T, kind items.Kind, container, video, codec string) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	service := f.server.items

	item, err := service.SaveScanned(ctx, items.Item{
		Kind:         kind,
		Key:          string(kind) + ":" + container + ":" + video + ":" + codec,
		Name:         "rip." + container,
		SortName:     "rip." + container,
		DateModified: apiutil.Ptr(time.Now()),
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	source, err := service.SaveSource(ctx, items.MediaSource{
		SourceID:     f.newDownloader(t),
		ItemID:       item.ID,
		Path:         "/media/rip." + container,
		Name:         "rip." + container,
		DateModified: apiutil.Ptr(time.Now()),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = service.SaveProbe(ctx, item, source, items.MediaSource{
		Container: container,
		Streams: []*items.MediaStream{
			{Index: 0, Kind: streammodel.KindVideo, Codec: video, Height: apiutil.Ptr(int32(1080)), Width: apiutil.Ptr(int32(1920))},
			{Index: 1, Kind: streammodel.KindAudio, Codec: codec},
		},
		Keyframes: f.keyframes,
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

	item, err := service.ItemByID(ctx, items.Everyone, id)
	if err != nil {
		t.Fatalf("failed to read the item: %v", err)
	}

	source, err := service.SaveSource(ctx, items.MediaSource{
		SourceID:     f.newDownloader(t),
		ItemID:       id,
		Path:         path,
		Name:         path,
		DateModified: apiutil.Ptr(time.Now()),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = service.SaveProbe(ctx, item, source, items.MediaSource{
		Container: strings.TrimPrefix(filepath.Ext(path), "."),
		Streams: []*items.MediaStream{
			{Index: 0, Kind: streammodel.KindVideo, Codec: video, Height: &height, Width: apiutil.Ptr(height * 16 / 9)},
			{Index: 1, Kind: streammodel.KindAudio, Codec: audio},
		},
	})
	if err != nil {
		t.Fatalf("failed to probe the source: %v", err)
	}
}

func (f *fixture) unscanned(t *testing.T) uuid.UUID {
	t.Helper()

	item, err := f.server.items.SaveScanned(context.Background(), items.Item{
		Kind:         itemmodel.KindMovie,
		Key:          "movie:unscanned",
		Name:         "unscanned",
		SortName:     "unscanned",
		DateModified: apiutil.Ptr(time.Now()),
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
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")

		source := fixture.source(t, id, firstPlay(&chrome))

		if apiutil.Deref(source.SupportsDirectPlay) || apiutil.Deref(source.SupportsDirectStream) {
			t.Error("the client was offered a second path to the same item")
		}
		if !apiutil.Deref(source.SupportsTranscoding) || source.TranscodingUrl == nil {
			t.Fatal("the client was left with nothing to play")
		}
		if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHls {
			t.Errorf("transcoding protocol = %q, want hls", got)
		}
	})

	t.Run("a file the client plays as it is is direct play as well as a url to it", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")

		source := fixture.source(t, id, firstPlay(&chrome))

		if !apiutil.Deref(source.SupportsDirectPlay) {
			t.Error("a file the client takes untouched was not offered as direct play, which a native player builds its own url for")
		}
		if apiutil.Deref(source.SupportsDirectStream) {
			t.Error("the file was offered as a direct stream too")
		}
		if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHttp {
			t.Errorf("transcoding protocol = %q, want the file over http", got)
		}
		if url := apiutil.Deref(source.TranscodingUrl); !strings.Contains(url, "/stream.mkv?") {
			t.Errorf("transcoding url = %q, want the file itself", url)
		}
	})

	t.Run("a file that failed to play is not offered as direct play again", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")

		if apiutil.Deref(fixture.source(t, id, afterPlaybackError(&chrome)).SupportsDirectPlay) {
			t.Error("direct play was offered to a client that asked for none")
		}
	})

	t.Run("a copy needs the file's keyframes to be cut into segments", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.keyframes = nil
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")

		source := fixture.source(t, id, firstPlay(&chrome))

		if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHttp {
			t.Errorf("transcoding protocol = %q, want a progressive stream until the file's keyframes are known", got)
		}
		if url := apiutil.Deref(source.TranscodingUrl); !strings.Contains(url, "/stream.mp4?") {
			t.Errorf("transcoding url = %q, want a progressive remux", url)
		}
	})

	t.Run("an encode needs no keyframes", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.keyframes = nil
		id := fixture.addRipped(t, itemmodel.KindMovie, "mkv", "mpeg4", "aac")

		if got := apiutil.Deref(fixture.source(t, id, firstPlay(&chrome)).TranscodingSubProtocol); got != api.MediaStreamProtocolHls {
			t.Errorf("transcoding protocol = %q, want hls", got)
		}
	})

	t.Run("a picture is never encoded down from above 1080p", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRipped(t, itemmodel.KindMovie, "mkv", "mpeg4", "aac")
		fixture.addCopy(t, id, "/media/uhd.mkv", "mpeg4", "aac", 2160)

		source := fixture.source(t, id, firstPlay(&chrome))
		if got := apiutil.Deref(source.Id); got != fixture.sourceID(t, id, "/media/rip.mkv") {
			t.Errorf("encoded the file %q, want the 1080p one", got)
		}

		only := fixture.addRipped(t, itemmodel.KindMovie, "avi", "mpeg4", "aac")
		if _, err := fixture.client.MediaStream.Update().
			Where(streammodel.HasSourceWith(sourcemodel.ItemID(only)), streammodel.KindEQ(streammodel.KindVideo)).
			SetHeight(int32(2160)).Save(context.Background()); err != nil {
			t.Fatalf("failed to make the file 4K: %v", err)
		}
		if got := apiutil.Deref(fixture.answer(t, only, firstPlay(&chrome)).ErrorCode); got != api.NoCompatibleStream {
			t.Errorf("error code = %q, want %q for a title whose only copy would be encoded down from 4K", got, api.NoCompatibleStream)
		}
	})

	t.Run("an item with several files answers with one of them", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")
		fixture.addCopy(t, id, "/media/second.mkv", "h264", "aac", 1080)

		if got := len(fixture.sources(t, id, firstPlay(&chrome))); got != 1 {
			t.Errorf("answered with %d sources, want the one the client should play", got)
		}
	})

	t.Run("the version the client names is not the client's to choose", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")
		fixture.addCopy(t, id, "/media/uhd.mkv", "h264", "aac", 2160)
		named := fixture.sourceID(t, id, "/media/rip.mkv")
		chosen := fixture.sourceID(t, id, "/media/uhd.mkv")

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
		if got := apiutil.Deref(sources[0].Id); got != chosen {
			t.Errorf("answered the file %q, want the one this server chose, %q", got, chosen)
		}
		if got := apiutil.Deref(sources[0].Path); got != "uhd.mkv" {
			t.Errorf("answered the path %q, want the file's name and no directory above it", got)
		}
	})

	t.Run("nothing is answered as a stream that has to be opened, looped or probed", func(t *testing.T) {
		fixture := newFixture(t)
		for _, id := range []uuid.UUID{
			fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3"),
			fixture.addRip(t, itemmodel.KindAudio, "flac", "flac"),
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

	t.Run("a song is answered with a url this server wrote", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindAudio, "flac", "flac")

		source := fixture.source(t, id, firstPlay(&chrome))

		if apiutil.Deref(source.SupportsDirectPlay) || apiutil.Deref(source.SupportsDirectStream) {
			t.Error("the client was offered a second path to the same song")
		}
		if !apiutil.Deref(source.SupportsTranscoding) {
			t.Fatal("a song was answered with no way to reach it, which jellyfin-web reports as no compatible stream")
		}
		if url := apiutil.Deref(source.TranscodingUrl); !strings.HasPrefix(url, "/Audio/"+id.String()+"/stream.") {
			t.Errorf("transcoding url = %q, want the audio route", url)
		}
	})

	t.Run("a song the client cannot decode is encoded into a container it can", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindAudio, "flac", "flac")

		source := fixture.source(t, id, firstPlay(&chrome))

		url := apiutil.Deref(source.TranscodingUrl)
		for _, want := range []string{"/stream.mp3?", "audioCodec=mp3", "container=mp3"} {
			if !strings.Contains(url, want) {
				t.Errorf("transcoding url = %q, want it to carry %q", url, want)
			}
		}
		if got := apiutil.Deref(source.TranscodingContainer); got != "mp3" {
			t.Errorf("transcoding container = %q, want mp3", got)
		}
	})

	t.Run("a song the client decodes is served as it is", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindAudio, "mp3", "mp3")

		url := apiutil.Deref(fixture.source(t, id, firstPlay(&chrome)).TranscodingUrl)

		for _, want := range []string{"/stream.mp3?", "audioCodec=mp3", "copyTimestamps=true"} {
			if !strings.Contains(url, want) {
				t.Errorf("transcoding url = %q, want it to carry %q", url, want)
			}
		}
	})

	t.Run("a song is encoded into what the client asked a transcode to be", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindAudio, "flac", "flac")
		profile := profileFrom(t, `{
			"DirectPlayProfiles": [{"Container": "mp3", "Type": "Audio"}],
			"TranscodingProfiles": [
				{"Container": "ts", "Type": "Audio", "AudioCodec": "aac", "Protocol": "hls"},
				{"Container": "opus", "Type": "Audio", "AudioCodec": "opus", "Protocol": "http"}
			]
		}`)

		url := apiutil.Deref(fixture.source(t, id, firstPlay(&profile)).TranscodingUrl)

		if !strings.Contains(url, "/stream.opus?") {
			t.Errorf("transcoding url = %q, want the http transcoding profile's opus rather than hls or the direct play mp3", url)
		}
	})

	t.Run("a song nothing the client takes can carry is no compatible stream", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindAudio, "flac", "flac")
		profile := profileFrom(t, `{"DirectPlayProfiles": [{"Container": "wav", "Type": "Audio"}]}`)

		if fixture.answer(t, id, firstPlay(&profile)).ErrorCode == nil {
			t.Error("a song no client container can carry was answered as playable")
		}
	})

	t.Run("audio the client cannot decode is answered in a container it can", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")

		source := fixture.source(t, id, firstPlay(&chrome))

		if got := apiutil.Deref(source.TranscodingContainer); got != "mp4" {
			t.Errorf("transcoding container = %q, want mp4", got)
		}
		url := apiutil.Deref(source.TranscodingUrl)
		if !strings.HasPrefix(url, "/Videos/"+id.String()+"/master.m3u8?") {
			t.Errorf("transcoding url = %q, want the item's master playlist", url)
		}
		if !strings.Contains(url, "audioCodec=aac") {
			t.Errorf("transcoding url = %q, want it to name the audio it will carry", url)
		}
		if !strings.Contains(url, "api_key=the-token") {
			t.Errorf("transcoding url carries no token: %q", url)
		}
	})

	t.Run("a source the client declared is named as the source", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")

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
		id := fixture.addRip(t, itemmodel.KindMovie, "avi", "aac")

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
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")

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
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")

		body := firstPlay(&chrome)
		body.StartTimeTicks = apiutil.Ptr(int64(25_000_000))

		if got := apiutil.Deref(fixture.source(t, id, body).TranscodingUrl); !strings.Contains(got, "startTimeTicks=25000000") {
			t.Errorf("transcoding url = %q, want the position the client seeked to", got)
		}
	})

	t.Run("a picture nothing declared can carry is handed over as an h264 encode", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRipped(t, itemmodel.KindMovie, "mkv", "mpeg4", "aac")

		source := fixture.source(t, id, firstPlay(&chrome))

		if apiutil.Deref(source.SupportsDirectPlay) || apiutil.Deref(source.SupportsDirectStream) {
			t.Error("an encode was offered as a direct play or a direct stream")
		}
		url := apiutil.Deref(source.TranscodingUrl)
		for _, want := range []string{"/master.m3u8?", "videoCodec=h264", "maxHeight=1080", "audioCodec=aac", "allowVideoStreamCopy=false"} {
			if !strings.Contains(url, want) {
				t.Errorf("transcoding url = %q, want it to carry %q", url, want)
			}
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
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")

		if fixture.answer(t, id, firstPlay(&chrome)).ErrorCode != nil {
			t.Error("a source that can be muxed was answered as no compatible stream")
		}
	})

	t.Run("transcoding and a url to transcode from always travel together", func(t *testing.T) {
		fixture := newFixture(t)
		ids := []uuid.UUID{
			fixture.addRipped(t, itemmodel.KindMovie, "mkv", "mpeg4", "aac"),
			fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3"),
			fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac"),
			fixture.addRip(t, itemmodel.KindAudio, "flac", "flac"),
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
		id := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")

		raw := strings.ToLower(apiutil.Deref(fixture.source(t, id, afterPlaybackError(&chrome)).TranscodingUrl))

		for _, refusal := range []string{"allowvideostreamcopy=false", "allowaudiostreamcopy=false"} {
			if !strings.Contains(raw, refusal) {
				t.Errorf("url = %q, want it to carry %q so jellyfin-web stops retrying", raw, refusal)
			}
		}
	})
}

func (f *fixture) newDownloader(t *testing.T) uuid.UUID {
	t.Helper()

	record, err := f.client.Source.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(downloadermodel.KindRadarr).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	t.Cleanup(func() {
		if err := f.client.Source.DeleteOne(record).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	return record.ID
}

func TestItemDtos_ListAFile(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	id := fixture.addRipped(t, itemmodel.KindEpisode, "mkv", "h264", "eac3")
	item, err := fixture.server.items.ItemByID(ctx, items.Everyone, id)
	if err != nil {
		t.Fatalf("failed to read the item back: %v", err)
	}

	converted, err := dto.ItemDtos(ctx, fixture.server.items, []*items.Item{item})
	if err != nil {
		t.Fatalf("failed to convert the item: %v", err)
	}

	if got := apiutil.Deref(converted[0].Path); got != "rip.mkv" {
		t.Errorf("the item names the path %q, want the file's name: Infuse fails a list whose items carry none, and the directories above it are the operator's", got)
	}

	listed := apiutil.Deref(converted[0].MediaSources)
	if len(listed) != 1 {
		t.Fatalf("media sources = %d, want the one file: the Android TV app reads the next episode's streams off its queue entry", len(listed))
	}
	if got := apiutil.Deref(listed[0].Id); got != id.String() {
		t.Errorf("the listed source is %q, want the item's own id %q: jellyfin-web fetches an item by the source id its detail page hands to play", got, id)
	}
	if streams := apiutil.Deref(listed[0].MediaStreams); len(streams) != 2 {
		t.Errorf("media streams = %d, want the picture and the sound", len(streams))
	}
	if streams := apiutil.Deref(converted[0].MediaStreams); len(streams) != 2 {
		t.Errorf("the item's own media streams = %d, want the same two", len(streams))
	}
}
