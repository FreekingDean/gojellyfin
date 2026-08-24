package mediainfo

import (
	"context"
	"io"
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

func TestServer_GetBitrateTestBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		size *int32
		want int64
	}{
		{name: "default", want: defaultBitrateTestSize},
		{name: "requested size", size: apiutil.Ptr(int32(4096)), want: 4096},
		{name: "capped", size: apiutil.Ptr(int32(maxBitrateTestSize + 1)), want: maxBitrateTestSize},
		{name: "zero", size: apiutil.Ptr(int32(0)), want: 1},
		{name: "negative", size: apiutil.Ptr(int32(-5)), want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := New(nil).GetBitrateTestBytes(context.Background(), api.GetBitrateTestBytesRequestObject{
				Params: api.GetBitrateTestBytesParams{Size: tc.size},
			})
			if err != nil {
				t.Fatal(err)
			}

			body := response.(api.GetBitrateTestBytes200ApplicationoctetStreamResponse)
			if body.ContentLength != tc.want {
				t.Errorf("got content length %d, want %d", body.ContentLength, tc.want)
			}

			written, err := io.Copy(io.Discard, body.Body)
			if err != nil {
				t.Fatal(err)
			}
			if written != tc.want {
				t.Errorf("got %d bytes, want %d", written, tc.want)
			}
		})
	}
}

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

	ctx := context.Background()
	service := f.server.items

	item, err := service.SaveScanned(ctx, items.Scanned{
		LibraryID:    f.library,
		Kind:         kind,
		Key:          string(kind) + ":" + container + ":" + codec,
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
			{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
			{Index: 1, Kind: streammodal.KindAudio, Codec: codec},
		},
	})
	if err != nil {
		t.Fatalf("failed to probe the source: %v", err)
	}

	return item.ID
}

func (f *fixture) playbackInfo(t *testing.T, id uuid.UUID, profile string) api.MediaSourceInfo {
	t.Helper()

	ctx := auth.ContextWithAuthorization(context.Background(), auth.Authorization{Token: "the-token"})
	request := api.GetPostedPlaybackInfoRequestObject{ItemId: id}
	if profile != "" {
		declared := profileFrom(t, profile)
		request.JSONBody = &api.GetPostedPlaybackInfoJSONRequestBody{DeviceProfile: &declared}
	}

	response, err := f.server.GetPostedPlaybackInfo(ctx, request)
	if err != nil {
		t.Fatalf("failed to answer playback info: %v", err)
	}

	sources := *api.PlaybackInfoResponse(response.(api.GetPostedPlaybackInfo200JSONResponse)).MediaSources
	if len(sources) != 1 {
		t.Fatalf("got %d media sources, want 1", len(sources))
	}

	return sources[0]
}

func TestServer_GetPostedPlaybackInfo(t *testing.T) {
	t.Run("refuses direct play of audio the client cannot decode", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3")

		source := fixture.playbackInfo(t, id, chromeProfile)

		if apiutil.Deref(source.SupportsDirectPlay) || apiutil.Deref(source.SupportsDirectStream) {
			t.Error("the client was told it can play a source it did not declare")
		}
		if !apiutil.Deref(source.SupportsTranscoding) {
			t.Error("the client was left with nothing to play")
		}
		if got := apiutil.Deref(source.TranscodingContainer); got != "mp4" {
			t.Errorf("transcoding container = %q, want mp4", got)
		}
		if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHttp {
			t.Errorf("transcoding protocol = %q, want http", got)
		}
		url := apiutil.Deref(source.TranscodingUrl)
		if !strings.HasPrefix(url, "/Videos/"+id.String()+"/stream.mp4?") {
			t.Errorf("transcoding url = %q", url)
		}
		if !strings.Contains(url, "api_key=the-token") {
			t.Errorf("transcoding url carries no token: %q", url)
		}
	})

	t.Run("direct plays audio the client declared", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "aac")

		source := fixture.playbackInfo(t, id, chromeProfile)

		if !apiutil.Deref(source.SupportsDirectStream) {
			t.Error("a source the client declared was refused")
		}
		if source.TranscodingUrl != nil {
			t.Errorf("transcoding url = %q, want none", *source.TranscodingUrl)
		}
	})

	t.Run("leaves a client that declared nothing alone", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindMovie, "mkv", "ac3")

		source := fixture.playbackInfo(t, id, "")

		if !apiutil.Deref(source.SupportsDirectStream) {
			t.Error("a client with no profile was refused direct play")
		}
	})

	t.Run("leaves a song alone", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, itemmodal.KindAudio, "flac", "flac")

		source := fixture.playbackInfo(t, id, chromeProfile)

		if source.TranscodingUrl != nil {
			t.Errorf("a song was sent for a video remux: %q", *source.TranscodingUrl)
		}
	})
}
