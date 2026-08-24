package subtitle

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

const srtFile = `1
00:00:01,000 --> 00:00:04,000
It's not my fault.

2
00:00:12,500 --> 00:00:15,000
Wake up.
Time to die.
`

type fixture struct {
	server    *Server
	client    *store.Client
	item      *items.Item
	source    uuid.UUID
	directory string
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
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.MediaStream.Delete().
			Where(streammodal.HasSourceWith(sourcemodal.HasItemWith(itemmodal.LibraryID(library.ID)))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the streams: %v", err)
		}
		if _, err := client.MediaSource.Delete().
			Where(sourcemodal.HasItemWith(itemmodal.LibraryID(library.ID))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the sources: %v", err)
		}
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	directory := t.TempDir()
	path := filepath.Join(directory, "Blade Runner (1982).mkv")
	item, err := client.Item.Create().
		SetLibraryID(library.ID).
		SetKind(itemmodal.KindMovie).
		SetName("Blade Runner").
		SetSortName("blade runner").
		SetPath(path).
		SetRunTimeTicks(250_000_000).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the item: %v", err)
	}

	source, err := client.MediaSource.Create().
		SetItemID(item.ID).
		SetName(filepath.Base(path)).
		SetPath(path).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the media source: %v", err)
	}

	return &fixture{
		server:    New(items.New(client), filesystem.New()),
		client:    client,
		item:      item,
		source:    source.ID,
		directory: directory,
	}
}

func (f *fixture) addSubtitle(t *testing.T, index int32, name, body string) {
	t.Helper()

	path := filepath.Join(f.directory, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", name, err)
	}

	err := f.client.MediaStream.Create().
		SetSourceID(f.source).
		SetIndex(index).
		SetKind(streammodal.KindSubtitle).
		SetCodec(strings.TrimPrefix(filepath.Ext(name), ".")).
		SetLanguage("eng").
		SetPath(path).
		SetIsExternal(true).
		Exec(context.Background())
	if err != nil {
		t.Fatalf("failed to create the stream: %v", err)
	}
}

func (f *fixture) addEmbedded(t *testing.T, index int32) {
	t.Helper()

	err := f.client.MediaStream.Create().
		SetSourceID(f.source).
		SetIndex(index).
		SetKind(streammodal.KindSubtitle).
		SetCodec("subrip").
		Exec(context.Background())
	if err != nil {
		t.Fatalf("failed to create the stream: %v", err)
	}
}

func body(t *testing.T, reader io.Reader) string {
	t.Helper()

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read the body: %v", err)
	}

	return string(content)
}

func TestServer_GetSubtitle(t *testing.T) {
	t.Run("serves what it can read", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		fixture.addSubtitle(t, 2, "Blade Runner (1982).en.srt", srtFile)

		tests := []struct {
			name            string
			index           int32
			format          string
			wantContentType string
			wantLength      int64
			wantBody        string
		}{
			{
				name:            "serves the file untouched when the format matches",
				index:           2,
				format:          "srt",
				wantContentType: "application/x-subrip",
				wantLength:      int64(len(srtFile)),
				wantBody:        srtFile,
			},
			{
				name:            "converts to webvtt",
				index:           2,
				format:          "vtt",
				wantContentType: "text/vtt",
				wantBody:        "WEBVTT\n\n00:00:01.000 --> 00:00:04.000\nIt's not my fault.\n\n00:00:12.500 --> 00:00:15.000\nWake up.\nTime to die.\n\n",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response, err := fixture.server.GetSubtitle(ctx, api.GetSubtitleRequestObject{
					RouteItemId: fixture.item.ID,
					RouteIndex:  test.index,
					RouteFormat: test.format,
				})
				if err != nil {
					t.Fatalf("failed to get the subtitle: %v", err)
				}

				result, ok := response.(api.GetSubtitle200TextResponse)
				if !ok {
					t.Fatalf("response = %T, want api.GetSubtitle200TextResponse", response)
				}
				if result.ContentType != test.wantContentType {
					t.Errorf("ContentType = %q, want %q", result.ContentType, test.wantContentType)
				}
				if result.ContentLength != test.wantLength {
					t.Errorf("ContentLength = %d, want %d", result.ContentLength, test.wantLength)
				}
				if got := body(t, result.Body); got != test.wantBody {
					t.Errorf("body = %q, want %q", got, test.wantBody)
				}
			})
		}
	})

	t.Run("needs ffmpeg", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		fixture.addEmbedded(t, 3)
		fixture.addSubtitle(t, 4, "Blade Runner (1982).fr.ass", "[Script Info]")

		tests := []struct {
			name  string
			index int32
		}{
			{name: "refuses to extract an embedded track", index: 3},
			{name: "refuses a conversion it cannot do", index: 4},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				_, err := fixture.server.GetSubtitle(ctx, api.GetSubtitleRequestObject{
					RouteItemId: fixture.item.ID,
					RouteIndex:  test.index,
					RouteFormat: "vtt",
				})
				if !errors.Is(err, api.ErrNotImplemented) {
					t.Errorf("err = %v, want api.ErrNotImplemented", err)
				}
			})
		}
	})

	t.Run("fails for an index that is not a subtitle", func(t *testing.T) {
		fixture := newFixture(t)

		_, err := fixture.server.GetSubtitle(context.Background(), api.GetSubtitleRequestObject{
			RouteItemId: fixture.item.ID,
			RouteIndex:  9,
			RouteFormat: "vtt",
		})
		if err == nil {
			t.Error("want an error for a missing stream")
		}
	})
}

func TestServer_GetSubtitleWithTicks(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	fixture.addSubtitle(t, 0, "Blade Runner (1982).en.srt", srtFile)

	copyTimestamps := true
	end := int64(50_000_000)

	tests := []struct {
		name     string
		format   string
		start    int64
		params   api.GetSubtitleWithTicksParams
		wantBody string
	}{
		{
			name:     "drops earlier cues and rebases the rest",
			format:   "vtt",
			start:    100_000_000,
			wantBody: "WEBVTT\n\n00:00:02.500 --> 00:00:05.000\nWake up.\nTime to die.\n\n",
		},
		{
			name:     "keeps the original timestamps when asked",
			format:   "srt",
			start:    100_000_000,
			params:   api.GetSubtitleWithTicksParams{CopyTimestamps: &copyTimestamps},
			wantBody: "1\n00:00:12,500 --> 00:00:15,000\nWake up.\nTime to die.\n\n",
		},
		{
			name:     "stops at the end of the window",
			format:   "vtt",
			params:   api.GetSubtitleWithTicksParams{EndPositionTicks: &end},
			wantBody: "WEBVTT\n\n00:00:01.000 --> 00:00:04.000\nIt's not my fault.\n\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := fixture.server.GetSubtitleWithTicks(ctx, api.GetSubtitleWithTicksRequestObject{
				RouteItemId:             fixture.item.ID,
				RouteIndex:              0,
				RouteFormat:             test.format,
				RouteStartPositionTicks: test.start,
				Params:                  test.params,
			})
			if err != nil {
				t.Fatalf("failed to get the subtitle: %v", err)
			}

			result, ok := response.(api.GetSubtitleWithTicks200TextResponse)
			if !ok {
				t.Fatalf("response = %T, want api.GetSubtitleWithTicks200TextResponse", response)
			}
			if got := body(t, result.Body); got != test.wantBody {
				t.Errorf("body = %q, want %q", got, test.wantBody)
			}
		})
	}
}

func TestServer_GetSubtitlePlaylist(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	fixture.addSubtitle(t, 0, "Blade Runner (1982).en.srt", srtFile)

	t.Run("segments the runtime", func(t *testing.T) {
		response, err := fixture.server.GetSubtitlePlaylist(ctx, api.GetSubtitlePlaylistRequestObject{
			ItemId: fixture.item.ID,
			Index:  0,
			Params: api.GetSubtitlePlaylistParams{SegmentLength: 10},
		})
		if err != nil {
			t.Fatalf("failed to get the playlist: %v", err)
		}

		result, ok := response.(api.GetSubtitlePlaylist200ApplicationxMpegURLResponse)
		if !ok {
			t.Fatalf("response = %T, want api.GetSubtitlePlaylist200ApplicationxMpegURLResponse", response)
		}

		want := strings.Join([]string{
			"#EXTM3U",
			"#EXT-X-TARGETDURATION:10",
			"#EXT-X-VERSION:3",
			"#EXT-X-MEDIA-SEQUENCE:0",
			"#EXT-X-PLAYLIST-TYPE:VOD",
			"#EXTINF:10.000,",
			"Stream.vtt?StartPositionTicks=0&EndPositionTicks=100000000&api_key=",
			"#EXTINF:10.000,",
			"Stream.vtt?StartPositionTicks=100000000&EndPositionTicks=200000000&api_key=",
			"#EXTINF:5.000,",
			"Stream.vtt?StartPositionTicks=200000000&EndPositionTicks=250000000&api_key=",
			"#EXT-X-ENDLIST",
			"",
		}, "\n")
		if got := body(t, result.Body); got != want {
			t.Errorf("playlist = %q, want %q", got, want)
		}
	})

	t.Run("refuses a track it cannot segment into webvtt", func(t *testing.T) {
		fixture.addEmbedded(t, 5)
		fixture.addSubtitle(t, 6, "Blade Runner (1982).fr.ass", "[Script Info]")

		for _, index := range []int32{5, 6} {
			_, err := fixture.server.GetSubtitlePlaylist(ctx, api.GetSubtitlePlaylistRequestObject{
				ItemId: fixture.item.ID,
				Index:  index,
				Params: api.GetSubtitlePlaylistParams{SegmentLength: 10},
			})
			if !errors.Is(err, api.ErrNotImplemented) {
				t.Errorf("index %d: err = %v, want api.ErrNotImplemented", index, err)
			}
		}
	})

	t.Run("404s for a stream that is not there", func(t *testing.T) {
		response, err := fixture.server.GetSubtitlePlaylist(ctx, api.GetSubtitlePlaylistRequestObject{
			ItemId: fixture.item.ID,
			Index:  7,
			Params: api.GetSubtitlePlaylistParams{SegmentLength: 10},
		})
		if err != nil {
			t.Fatalf("failed to get the playlist: %v", err)
		}

		if _, ok := response.(api.GetSubtitlePlaylist404JSONResponse); !ok {
			t.Errorf("response = %T, want api.GetSubtitlePlaylist404JSONResponse", response)
		}
	})
}
