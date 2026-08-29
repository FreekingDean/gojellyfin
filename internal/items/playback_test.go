package items

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

var chrome = Capabilities{
	Profiles: []Profile{
		{Container: "webm", VideoCodec: "vp8,vp9,av1", AudioCodec: "vorbis,opus"},
		{Container: "mp4,m4v", VideoCodec: "h264,vp8,vp9,av1", AudioCodec: "aac,mp3,opus,flac,vorbis"},
		{Container: "mkv", VideoCodec: "h264,vp8,vp9,av1", AudioCodec: "aac,mp3,opus,flac,vorbis"},
	},
	Conditions: []Condition{
		{Codec: "h264", Property: "IsAnamorphic", Verb: "NotEquals", Value: "true"},
		{Codec: "h264", Property: "VideoProfile", Verb: "EqualsAny", Value: "high|main|baseline|constrained baseline"},
		{Codec: "h264", Property: "VideoRangeType", Verb: "EqualsAny", Value: "SDR"},
		{Codec: "h264", Property: "VideoLevel", Verb: "LessThanEqual", Value: "52"},
		{Codec: "h264", Property: "IsInterlaced", Verb: "NotEquals", Value: "true"},
	},
}

var web = Capabilities{Profiles: []Profile{
	{Container: "webm", VideoCodec: "vp8,vp9,av1", AudioCodec: "opus"},
	{
		Container:  "mp4,m4v",
		VideoCodec: "h264,hevc,mpeg2video,vc1,msmpeg4v2,vp9,av1",
		AudioCodec: "aac,mp3,ac3,eac3,mp2,dca,dts,pcm_s16le,pcm_s24le,truehd,aac_latm,opus,flac,alac,vorbis",
	},
}}

var stick = Capabilities{Profiles: []Profile{
	{Container: "ts", VideoCodec: "h264", AudioCodec: "aac"},
}}

func rip(container string, audio ...string) *MediaSource {
	return ripped(container, "h264", 1080, audio...)
}

func ripped(container, video string, height int32, audio ...string) *MediaSource {
	source := &MediaSource{ID: uuid.New(), Container: container}
	if video != "" {
		source.Edges.Streams = []*MediaStream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: video, Height: height, Width: height * 16 / 9},
		}
	}
	for index, codec := range audio {
		source.Edges.Streams = append(source.Edges.Streams, &MediaStream{
			Index: int32(index + 1),
			Kind:  streammodal.KindAudio,
			Codec: codec,
		})
	}

	return source
}

func hdr(source *MediaSource) *MediaSource {
	source.Edges.Streams[0].VideoRangeType = streammodal.VideoRangeTypeHDR10

	return source
}

func TestPlanFor(t *testing.T) {
	for _, tc := range []struct {
		name       string
		can        Capabilities
		source     *MediaSource
		change     Change
		container  string
		audioCodec string
	}{
		{
			name:       "everything declared is handed over as it is",
			can:        chrome,
			source:     rip("mkv", "aac"),
			change:     ChangeNone,
			container:  "mkv",
			audioCodec: "aac",
		},
		{
			name:       "codecs declared and the container not costs a mux and nothing else",
			can:        chrome,
			source:     rip("avi", "aac"),
			change:     ChangeContainer,
			container:  "mp4",
			audioCodec: "aac",
		},
		{
			name:       "audio the client cannot decode is the one stream converted",
			can:        chrome,
			source:     rip("mkv", "ac3"),
			change:     ChangeAudio,
			container:  "mp4",
			audioCodec: "aac",
		},
		{
			name:       "a client declaring another container gets that one",
			can:        stick,
			source:     rip("mkv", "ac3"),
			change:     ChangeAudio,
			container:  "ts",
			audioCodec: "aac",
		},
		{
			name:       "a client that declared nothing keeps the source",
			source:     rip("mkv", "ac3"),
			change:     ChangeNone,
			container:  "mkv",
			audioCodec: "ac3",
		},
		{
			name:       "an HDR picture where the client decodes SDR only",
			can:        chrome,
			source:     hdr(rip("mkv", "aac")),
			change:     ChangeVideo,
			container:  "mkv",
			audioCodec: "aac",
		},
		{
			name:       "an HDR picture beside audio the client also cannot decode",
			can:        chrome,
			source:     hdr(rip("mkv", "ac3")),
			change:     ChangeVideoAudio,
			container:  "mkv",
			audioCodec: "ac3",
		},
		{
			name:       "a picture nothing declared can carry needs the one encode nothing does",
			can:        chrome,
			source:     ripped("mkv", "mpeg4", 1080, "aac"),
			change:     ChangeVideo,
			container:  "mkv",
			audioCodec: "aac",
		},
		{
			name:       "a picture and audio nothing declared can carry needs both",
			can:        chrome,
			source:     ripped("mkv", "mpeg4", 1080, "ac3"),
			change:     ChangeVideoAudio,
			container:  "mkv",
			audioCodec: "ac3",
		},
		{
			name:       "a picture the real client never names",
			can:        web,
			source:     ripped("mkv", "theora", 1080, "aac"),
			change:     ChangeVideo,
			container:  "mkv",
			audioCodec: "aac",
		},
		{
			name:       "a file nobody probed is not held to codecs nobody read",
			can:        chrome,
			source:     ripped("mkv", "", 0),
			change:     ChangeNone,
			container:  "mkv",
			audioCodec: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := tc.can.plan(tc.source)

			if plan.Change != tc.change {
				t.Errorf("change = %v, want %v", plan.Change, tc.change)
			}
			if plan.Container != tc.container {
				t.Errorf("container = %q, want %q", plan.Container, tc.container)
			}
			if plan.AudioCodec != tc.audioCodec {
				t.Errorf("audio codec = %q, want %q", plan.AudioCodec, tc.audioCodec)
			}
		})
	}

	t.Run("the names the client writes are the names the probe wrote", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			source *MediaSource
			want   Change
		}{
			{name: "hevc, which the client spells the same way", source: ripped("mp4", "hevc", 2160, "aac"), want: ChangeNone},
			{name: "mpeg2video, not mpeg2", source: ripped("mp4", "mpeg2video", 576, "aac"), want: ChangeNone},
			{name: "msmpeg4v2, which is the one the client names", source: ripped("mp4", "msmpeg4v2", 480, "aac"), want: ChangeNone},
			{name: "vc1", source: ripped("mp4", "vc1", 1080, "aac"), want: ChangeNone},
			{name: "dca, which the client lists beside dts itself", source: ripped("mp4", "h264", 1080, "dca"), want: ChangeNone},
			{name: "eac3, not ec-3", source: ripped("mp4", "h264", 1080, "eac3"), want: ChangeNone},
			{name: "truehd", source: ripped("mp4", "h264", 1080, "truehd"), want: ChangeNone},
			{name: "aac_latm", source: ripped("mp4", "h264", 1080, "aac_latm"), want: ChangeNone},
			{name: "pcm_s16le", source: ripped("mp4", "h264", 1080, "pcm_s16le"), want: ChangeNone},
			{name: "a profile shouting its codec", source: ripped("mp4", "HEVC", 2160, "AAC"), want: ChangeNone},
			{name: "a picture the client never named", source: ripped("mp4", "theora", 1080, "aac"), want: ChangeVideo},
			{name: "audio the client never named", source: ripped("mp4", "h264", 1080, "wmav2"), want: ChangeAudio},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := web.plan(tc.source).Change; got != tc.want {
					t.Errorf("change = %v, want %v", got, tc.want)
				}
			})
		}
	})

	t.Run("a profile that named no codecs named none it refuses", func(t *testing.T) {
		open := Capabilities{Profiles: []Profile{{Container: "mp4"}}}

		if got := open.plan(rip("mp4", "ac3")).Change; got != ChangeNone {
			t.Errorf("change = %v, want %v", got, ChangeNone)
		}
	})

	t.Run("a container the client declared that cannot be written down a pipe is passed over", func(t *testing.T) {
		can := Capabilities{Profiles: []Profile{
			{Container: "webm", VideoCodec: "h264", AudioCodec: "opus"},
			{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"},
		}}

		if got := can.plan(rip("mkv", "ac3")).Container; got != "mp4" {
			t.Errorf("container = %q, want mp4", got)
		}
	})

	t.Run("a picture only an unwritable container declared is still carried", func(t *testing.T) {
		can := Capabilities{Profiles: []Profile{{Container: "webm", VideoCodec: "vp9", AudioCodec: "opus"}}}

		plan := can.plan(ripped("mkv", "vp9", 1080, "ac3"))

		if plan.Change != ChangeAudio {
			t.Errorf("change = %v, want %v", plan.Change, ChangeAudio)
		}
		if plan.Container != "mp4" {
			t.Errorf("container = %q, want mp4", plan.Container)
		}
	})
}

func TestService_SourceForOrder(t *testing.T) {
	ctx := context.Background()

	type copy struct {
		path   string
		video  string
		audio  string
		height int32
		hdr    bool
	}

	for _, tc := range []struct {
		name    string
		copies  []copy
		want    string
		change  Change
		refused bool
	}{
		{
			name:   "a) the 4K plays untouched",
			copies: []copy{{path: "/uhd.mkv", video: "h264", audio: "aac", height: 2160}, {path: "/hd.mkv", video: "h264", audio: "aac", height: 1080}},
			want:   "/uhd.mkv",
			change: ChangeNone,
		},
		{
			name:   "b) the 4K needs a mux and still comes first",
			copies: []copy{{path: "/uhd.avi", video: "h264", audio: "aac", height: 2160}, {path: "/hd.mkv", video: "h264", audio: "aac", height: 1080}},
			want:   "/uhd.avi",
			change: ChangeContainer,
		},
		{
			name:   "c) the 4K needs its audio converted and still beats a 1080p that does not",
			copies: []copy{{path: "/uhd.mkv", video: "h264", audio: "ac3", height: 2160}, {path: "/hd.mkv", video: "h264", audio: "aac", height: 1080}},
			want:   "/uhd.mkv",
			change: ChangeAudio,
		},
		{
			name:   "d) the 1080p plays untouched once the 4K needs a picture encode",
			copies: []copy{{path: "/uhd.mkv", video: "mpeg4", audio: "aac", height: 2160}, {path: "/hd.mkv", video: "h264", audio: "aac", height: 1080}},
			want:   "/hd.mkv",
			change: ChangeNone,
		},
		{
			name:   "e) the 1080p needs a mux",
			copies: []copy{{path: "/uhd.mkv", video: "mpeg4", audio: "aac", height: 2160}, {path: "/hd.avi", video: "h264", audio: "aac", height: 1080}},
			want:   "/hd.avi",
			change: ChangeContainer,
		},
		{
			name:   "f) the 1080p needs its audio converted",
			copies: []copy{{path: "/uhd.mkv", video: "mpeg4", audio: "aac", height: 2160}, {path: "/hd.mkv", video: "h264", audio: "ac3", height: 1080}},
			want:   "/hd.mkv",
			change: ChangeAudio,
		},
		{
			name:    "g) a picture encode with audio the client decodes is nothing this can serve",
			copies:  []copy{{path: "/only.mkv", video: "mpeg4", audio: "aac", height: 1080}},
			refused: true,
		},
		{
			name:    "h) a picture encode with audio it does not is no different",
			copies:  []copy{{path: "/only.mkv", video: "mpeg4", audio: "ac3", height: 1080}},
			refused: true,
		},
		{
			name:   "an HDR 4K falls through to the SDR 1080p beside it",
			copies: []copy{{path: "/uhd.mkv", video: "h264", audio: "aac", height: 2160, hdr: true}, {path: "/hd.mkv", video: "h264", audio: "aac", height: 1080}},
			want:   "/hd.mkv",
			change: ChangeNone,
		},
		{
			name:    "an HDR 4K with nothing beside it",
			copies:  []copy{{path: "/only.mkv", video: "h264", audio: "aac", height: 2160, hdr: true}},
			refused: true,
		},
		{
			name:   "a file nobody probed is still answered",
			copies: []copy{{path: "/only.mkv"}},
			want:   "/only.mkv",
			change: ChangeNone,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newFixture(t)
			film := fixture.film(t, tc.name)
			for _, file := range tc.copies {
				rangeType := VideoRangeType("")
				if file.hdr {
					rangeType = streammodal.VideoRangeTypeHDR10
				}
				fixture.copyRanged(t, film, file.path, file.video, file.audio, file.height, rangeType)
			}

			plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome)
			if tc.refused {
				if !errors.Is(err, ErrNoPlayable) {
					t.Fatalf("err = %v, want %v", err, ErrNoPlayable)
				}
				return
			}
			if err != nil {
				t.Fatalf("nothing was chosen: %v", err)
			}
			if plan.Source.Path != tc.want {
				t.Errorf("chose %q, want %q", plan.Source.Path, tc.want)
			}
			if plan.Change != tc.change {
				t.Errorf("change = %v, want %v", plan.Change, tc.change)
			}
		})
	}

	t.Run("the richer encode of two at one resolution", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:bitrates")
		fixture.copy(t, film, "/thin.mkv", "h264", "aac", 1080)
		fixture.copy(t, film, "/fat.mkv", "h264", "aac", 1080)
		fixture.bitrate(t, film, "/fat.mkv", 20_000_000)

		plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome)
		if err != nil {
			t.Fatalf("nothing was chosen: %v", err)
		}
		if plan.Source.Path != "/fat.mkv" {
			t.Errorf("chose %q, want the richer encode", plan.Source.Path)
		}
	})
}

func (f *fixture) copy(t *testing.T, item *Item, path, video, audio string, height int32) {
	t.Helper()

	f.copyRanged(t, item, path, video, audio, height, "")
}

func (f *fixture) copyRanged(t *testing.T, item *Item, path, video, audio string, height int32, rangeType VideoRangeType) {
	t.Helper()

	ctx := context.Background()
	source, err := f.service.SaveSource(ctx, ScannedSource{
		LibraryID: f.libraryID,
		ItemID:    item.ID,
		Path:      path,
		Name:      path,
	})
	if err != nil {
		t.Fatalf("failed to save the source of %q: %v", path, err)
	}

	streams := []Stream{{
		Index:     0,
		Kind:      streammodal.KindVideo,
		Codec:     video,
		Height:    height,
		Width:     height * 16 / 9,
		RangeType: rangeType,
	}}
	if audio != "" {
		streams = append(streams, Stream{Index: 1, Kind: streammodal.KindAudio, Codec: audio})
	}

	probe := Probe{Container: strings.TrimPrefix(filepath.Ext(path), "."), Streams: streams}
	if err := f.service.SaveProbe(ctx, item, source, probe); err != nil {
		t.Fatalf("failed to probe %q: %v", path, err)
	}
}

func (f *fixture) bitrate(t *testing.T, item *Item, path string, bitrate int32) {
	t.Helper()

	sources, err := f.service.MediaSources(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("failed to read the sources: %v", err)
	}
	for _, source := range sources {
		if source.Path != path {
			continue
		}
		if err := f.service.store.MediaSource.UpdateOne(source).SetBitrate(bitrate).Exec(context.Background()); err != nil {
			t.Fatalf("failed to set the bitrate: %v", err)
		}
	}
}

func (f *fixture) sourceID(t *testing.T, item *Item, path string) uuid.UUID {
	t.Helper()

	sources, err := f.service.MediaSources(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("failed to read the sources: %v", err)
	}
	for _, source := range sources {
		if source.Path == path {
			return source.ID
		}
	}
	t.Fatalf("the fixture has no source at %q", path)

	return uuid.Nil
}

func (f *fixture) film(t *testing.T, key string) *Item {
	t.Helper()

	item, err := f.service.SaveScanned(context.Background(), Scanned{
		LibraryID:    f.libraryID,
		Kind:         itemmodal.KindMovie,
		Key:          key,
		Name:         key,
		SortName:     key,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save %q: %v", key, err)
	}

	return item
}

func TestService_SourceFor(t *testing.T) {
	ctx := context.Background()

	t.Run("answers with one file and never the others", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:versions")
		fixture.copy(t, film, "/media/uhd.mkv", "h264", "aac", 2160)
		fixture.copy(t, film, "/media/hd.mkv", "h264", "aac", 1080)

		sources, err := fixture.service.MediaSources(ctx, film.ID)
		if err != nil || len(sources) != 2 {
			t.Fatalf("the fixture has %d sources: %v", len(sources), err)
		}

		plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome)
		if err != nil {
			t.Fatalf("failed to choose a source: %v", err)
		}
		if plan.Source.Path != "/media/uhd.mkv" {
			t.Errorf("chose %q, want the taller copy", plan.Source.Path)
		}
	})

	t.Run("prefers the taller picture over the cheaper one to serve", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:tiers")
		fixture.copy(t, film, "/media/uhd.mkv", "h264", "ac3", 2160)
		fixture.copy(t, film, "/media/hd.mkv", "h264", "aac", 1080)

		plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome)
		if err != nil {
			t.Fatalf("failed to choose a source: %v", err)
		}
		if plan.Source.Path != "/media/uhd.mkv" {
			t.Errorf("chose %q, want the 4K that only needs its audio converted", plan.Source.Path)
		}
		if plan.Change != ChangeAudio {
			t.Errorf("change = %v, want %v", plan.Change, ChangeAudio)
		}
	})

	t.Run("drops a copy whose picture would have to be re-encoded", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:oddball")
		fixture.copy(t, film, "/media/uhd.mkv", "mpeg4", "aac", 2160)
		fixture.copy(t, film, "/media/hd.mkv", "h264", "aac", 1080)

		plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome)
		if err != nil {
			t.Fatalf("failed to choose a source: %v", err)
		}
		if plan.Source.Path != "/media/hd.mkv" {
			t.Errorf("chose %q, want the copy that needs no picture encode", plan.Source.Path)
		}
	})

	t.Run("refuses when every copy would need its picture re-encoded", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:unencodable")
		fixture.copy(t, film, "/media/only.mkv", "mpeg4", "aac", 1080)

		if _, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome); !errors.Is(err, ErrNoPlayable) {
			t.Errorf("err = %v, want %v", err, ErrNoPlayable)
		}
	})

	t.Run("an SDR client gets the 1080p rather than the HDR 4K beside it", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:hdr-tiers")
		fixture.copyRanged(t, film, "/media/uhd.mkv", "h264", "aac", 2160, streammodal.VideoRangeTypeHDR10)
		fixture.copyRanged(t, film, "/media/hd.mkv", "h264", "aac", 1080, streammodal.VideoRangeTypeSDR)

		plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome)
		if err != nil {
			t.Fatalf("failed to choose a source: %v", err)
		}
		if plan.Source.Path != "/media/hd.mkv" {
			t.Errorf("chose %q, want the SDR copy the client can actually decode", plan.Source.Path)
		}
		if plan.Change != ChangeNone {
			t.Errorf("change = %v, want the SDR copy handed over untouched", plan.Change)
		}
	})

	t.Run("refuses when the only copy is one the client cannot decode", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:hdr-only")
		fixture.copyRanged(t, film, "/media/only.mkv", "h264", "aac", 2160, streammodal.VideoRangeTypeHDR10)

		if _, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome); !errors.Is(err, ErrNoPlayable) {
			t.Errorf("err = %v, want %v", err, ErrNoPlayable)
		}
	})

	t.Run("a copy the probe never read the range of is still offered", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:unread-range")
		fixture.copy(t, film, "/media/only.mkv", "h264", "aac", 1080)

		plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome)
		if err != nil {
			t.Fatalf("a file whose range nobody probed was refused: %v", err)
		}
		if plan.Change != ChangeNone {
			t.Errorf("change = %v, want %v", plan.Change, ChangeNone)
		}
	})

	t.Run("refuses an item with no file behind it", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:nofile")

		if _, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, chrome); !errors.Is(err, ErrNoSource) {
			t.Errorf("err = %v, want %v", err, ErrNoSource)
		}
	})

	t.Run("hands a client that declared nothing its own file", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:silent-client")
		fixture.copy(t, film, "/media/only.mkv", "mpeg4", "ac3", 1080)

		plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.Nil, Capabilities{})
		if err != nil {
			t.Fatalf("failed to choose a source: %v", err)
		}
		if plan.Change != ChangeNone {
			t.Errorf("change = %v, want %v", plan.Change, ChangeNone)
		}
	})
}

func TestService_SourceForNamed(t *testing.T) {
	ctx := context.Background()

	t.Run("the file the client named is the file it is planned", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:named")
		fixture.copy(t, film, "/uhd.mkv", "h264", "aac", 2160)
		fixture.copy(t, film, "/hd.mkv", "h264", "aac", 1080)

		plan, err := fixture.service.SourceFor(ctx, film.ID, fixture.sourceID(t, film, "/hd.mkv"), chrome)
		if err != nil {
			t.Fatalf("nothing was chosen: %v", err)
		}
		if plan.Source.Path != "/hd.mkv" {
			t.Errorf("chose %q, want the version the client named", plan.Source.Path)
		}
	})

	t.Run("a file the client named that cannot be served is not swapped for the one beside it", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:named-refused")
		fixture.copy(t, film, "/uhd.mkv", "mpeg4", "aac", 2160)
		fixture.copy(t, film, "/hd.mkv", "h264", "aac", 1080)

		_, err := fixture.service.SourceFor(ctx, film.ID, fixture.sourceID(t, film, "/uhd.mkv"), chrome)
		if !errors.Is(err, ErrNoPlayable) {
			t.Errorf("err = %v, want %v rather than a version the client did not ask for", err, ErrNoPlayable)
		}
	})

	t.Run("an id the item does not carry is answered as if the client named none", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:named-missing")
		fixture.copy(t, film, "/uhd.mkv", "h264", "aac", 2160)
		fixture.copy(t, film, "/hd.mkv", "h264", "aac", 1080)

		plan, err := fixture.service.SourceFor(ctx, film.ID, uuid.New(), chrome)
		if err != nil {
			t.Fatalf("nothing was chosen: %v", err)
		}
		if plan.Source.Path != "/uhd.mkv" {
			t.Errorf("chose %q, want the tallest", plan.Source.Path)
		}
	})
}
