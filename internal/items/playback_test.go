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

// What a browser declares. Kept narrow so a case that turns on one codec is
// obvious from the table rather than from reading the profile.
var chrome = Capabilities{Profiles: []Profile{
	{Container: "webm", VideoCodec: "vp8,vp9,av1", AudioCodec: "vorbis,opus"},
	{Container: "mp4,m4v", VideoCodec: "h264,vp8,vp9,av1", AudioCodec: "aac,mp3,opus,flac,vorbis"},
	{Container: "mkv", VideoCodec: "h264,vp8,vp9,av1", AudioCodec: "aac,mp3,opus,flac,vorbis"},
}}

// The codec names jellyfin-web actually puts in its DirectPlayProfiles, read
// off the vendored bundle rather than off the documentation, beside what
// ffprobe reports for the same codec. They are the same strings, which is why
// nothing here translates between the two vocabularies.
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

func TestPlanFor(t *testing.T) {
	for _, tc := range []struct {
		name       string
		can        Capabilities
		source     *MediaSource
		change     Change
		container  string
		videoCodec string
		audioCodec string
	}{
		{
			name:       "everything declared is handed over as it is",
			can:        chrome,
			source:     rip("mkv", "aac"),
			change:     ChangeNone,
			container:  "mkv",
			videoCodec: "h264",
			audioCodec: "aac",
		},
		{
			name:       "codecs declared and the container not costs a mux and nothing else",
			can:        chrome,
			source:     rip("avi", "aac"),
			change:     ChangeContainer,
			container:  "mp4",
			videoCodec: "h264",
			audioCodec: "aac",
		},
		{
			name:       "audio the client cannot decode is the one stream converted",
			can:        chrome,
			source:     rip("mkv", "ac3"),
			change:     ChangeAudio,
			container:  "mp4",
			videoCodec: "h264",
			audioCodec: "aac",
		},
		{
			name:       "a client declaring another container gets that one",
			can:        stick,
			source:     rip("mkv", "ac3"),
			change:     ChangeAudio,
			container:  "ts",
			videoCodec: "h264",
			audioCodec: "aac",
		},
		{
			name:       "a client that declared nothing keeps the source",
			source:     rip("mkv", "ac3"),
			change:     ChangeNone,
			container:  "mkv",
			videoCodec: "h264",
			audioCodec: "ac3",
		},
		{
			name:       "a picture nothing declared can carry needs the one encode nothing does",
			can:        chrome,
			source:     ripped("mkv", "mpeg4", 1080, "aac"),
			change:     ChangeVideo,
			container:  "mkv",
			videoCodec: "mpeg4",
			audioCodec: "aac",
		},
		{
			name:       "a picture and audio nothing declared can carry needs both",
			can:        chrome,
			source:     ripped("mkv", "mpeg4", 1080, "ac3"),
			change:     ChangeVideoAudio,
			container:  "mkv",
			videoCodec: "mpeg4",
			audioCodec: "ac3",
		},
		{
			name:       "a picture the real client never names",
			can:        web,
			source:     ripped("mkv", "theora", 1080, "aac"),
			change:     ChangeVideo,
			container:  "mkv",
			videoCodec: "theora",
			audioCodec: "aac",
		},
		{
			name:       "a file nobody probed is not held to codecs nobody read",
			can:        chrome,
			source:     ripped("mkv", "", 0),
			change:     ChangeNone,
			container:  "mkv",
			videoCodec: "",
			audioCodec: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := tc.can.planFor(tc.source)

			if plan.Change != tc.change {
				t.Errorf("change = %v, want %v", plan.Change, tc.change)
			}
			if plan.Container != tc.container {
				t.Errorf("container = %q, want %q", plan.Container, tc.container)
			}
			if plan.VideoCodec != tc.videoCodec {
				t.Errorf("video codec = %q, want %q", plan.VideoCodec, tc.videoCodec)
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
				if got := web.planFor(tc.source).Change; got != tc.want {
					t.Errorf("change = %v, want %v", got, tc.want)
				}
			})
		}
	})

	t.Run("a profile that named no codecs named none it refuses", func(t *testing.T) {
		open := Capabilities{Profiles: []Profile{{Container: "mp4"}}}

		if got := open.planFor(rip("mp4", "ac3")).Change; got != ChangeNone {
			t.Errorf("change = %v, want %v", got, ChangeNone)
		}
	})

	t.Run("a container the client declared that cannot be written down a pipe is passed over", func(t *testing.T) {
		can := Capabilities{Profiles: []Profile{
			{Container: "webm", VideoCodec: "h264", AudioCodec: "opus"},
			{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"},
		}}

		if got := can.planFor(rip("mkv", "ac3")).Container; got != "mp4" {
			t.Errorf("container = %q, want mp4", got)
		}
	})

	t.Run("a picture only an unwritable container declared is still carried", func(t *testing.T) {
		can := Capabilities{Profiles: []Profile{{Container: "webm", VideoCodec: "vp9", AudioCodec: "opus"}}}

		plan := can.planFor(ripped("mkv", "vp9", 1080, "ac3"))

		if plan.Change != ChangeAudio {
			t.Errorf("change = %v, want %v", plan.Change, ChangeAudio)
		}
		if plan.Container != "mp4" {
			t.Errorf("container = %q, want mp4", plan.Container)
		}
	})
}

// The order Dean specified, read back as the eight rows he wrote it as:
// resolution decides first and what has to change breaks the tie, except that
// a picture needing an encode ranks below everything and only at the shortest
// source it applies to.
func TestRanked(t *testing.T) {
	uhd := func(change Change) Plan { return planAt(2160, change) }
	hd := func(change Change) Plan { return planAt(1080, change) }

	for _, tc := range []struct {
		name  string
		plans []Plan
		want  Plan
	}{
		{name: "a) 4k untouched over everything", plans: []Plan{hd(ChangeNone), uhd(ChangeNone)}, want: uhd(ChangeNone)},
		{name: "b) 4k muxed over 4k audio converted", plans: []Plan{uhd(ChangeAudio), uhd(ChangeContainer)}, want: uhd(ChangeContainer)},
		{name: "c) 4k audio converted over 1080 untouched", plans: []Plan{hd(ChangeNone), uhd(ChangeAudio)}, want: uhd(ChangeAudio)},
		{name: "d) 1080 untouched when 4k needs a picture encode", plans: []Plan{uhd(ChangeVideo), hd(ChangeNone)}, want: hd(ChangeNone)},
		{name: "e) 1080 muxed over 1080 audio converted", plans: []Plan{hd(ChangeAudio), hd(ChangeContainer)}, want: hd(ChangeContainer)},
		{name: "f) 1080 audio converted over any picture encode", plans: []Plan{uhd(ChangeVideo), hd(ChangeAudio)}, want: hd(ChangeAudio)},
		{name: "g) the shortest picture encode, not the tallest", plans: []Plan{uhd(ChangeVideo), hd(ChangeVideo)}, want: hd(ChangeVideo)},
		{name: "h) audio comes with the picture only when it has to", plans: []Plan{hd(ChangeVideoAudio), hd(ChangeVideo)}, want: hd(ChangeVideo)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ranked(tc.plans)

			if got.Change != tc.want.Change || got.height() != tc.want.height() {
				t.Errorf("chose %dp %v, want %dp %v", got.height(), got.Change, tc.want.height(), tc.want.Change)
			}
		})
	}

	t.Run("a file nobody probed is not ranked below one that was", func(t *testing.T) {
		unprobed := Plan{Source: ripped("mkv", "", 0)}
		probed := Plan{Source: ripped("mkv", "h264", 1080, "ac3"), Change: ChangeAudio}

		if got := ranked([]Plan{probed, unprobed}); got.Source != unprobed.Source {
			t.Error("a source the probe has not reached tiered as a short one and lost to an encode")
		}
	})

	t.Run("the richer encode of two at one resolution", func(t *testing.T) {
		thin := Plan{Source: ripped("mkv", "h264", 1080, "aac")}
		fat := Plan{Source: ripped("mkv", "h264", 1080, "aac")}
		fat.Source.Bitrate = 20_000_000
		thin.Source.Bitrate = 4_000_000

		if got := ranked([]Plan{thin, fat}); got.Source != fat.Source {
			t.Error("the thinner encode was chosen at the same resolution and cost")
		}
	})
}

func planAt(height int32, change Change) Plan {
	return Plan{Source: ripped("mkv", "h264", height, "aac"), Change: change}
}

func (f *fixture) copy(t *testing.T, item *Item, path, video, audio string, height int32) {
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

	streams := []Stream{{Index: 0, Kind: streammodal.KindVideo, Codec: video, Height: height, Width: height * 16 / 9}}
	if audio != "" {
		streams = append(streams, Stream{Index: 1, Kind: streammodal.KindAudio, Codec: audio})
	}

	probe := Probe{Container: strings.TrimPrefix(filepath.Ext(path), "."), Streams: streams}
	if err := f.service.SaveProbe(ctx, item, source, probe); err != nil {
		t.Fatalf("failed to probe %q: %v", path, err)
	}
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

		plan, err := fixture.service.SourceFor(ctx, film.ID, chrome)
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

		plan, err := fixture.service.SourceFor(ctx, film.ID, chrome)
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

		plan, err := fixture.service.SourceFor(ctx, film.ID, chrome)
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

		if _, err := fixture.service.SourceFor(ctx, film.ID, chrome); !errors.Is(err, ErrNoPlayable) {
			t.Errorf("err = %v, want %v", err, ErrNoPlayable)
		}
	})

	t.Run("refuses an item with no file behind it", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:nofile")

		if _, err := fixture.service.SourceFor(ctx, film.ID, chrome); !errors.Is(err, ErrNoSource) {
			t.Errorf("err = %v, want %v", err, ErrNoSource)
		}
	})

	t.Run("hands a client that declared nothing its own file", func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.film(t, "movie:silent-client")
		fixture.copy(t, film, "/media/only.mkv", "mpeg4", "ac3", 1080)

		plan, err := fixture.service.SourceFor(ctx, film.ID, Capabilities{})
		if err != nil {
			t.Fatalf("failed to choose a source: %v", err)
		}
		if plan.Change != ChangeNone {
			t.Errorf("change = %v, want %v", plan.Change, ChangeNone)
		}
	})
}
