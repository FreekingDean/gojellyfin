package items

import (
	"testing"

	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

func picture(apply func(*MediaStream)) *MediaStream {
	stream := &MediaStream{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"}
	if apply != nil {
		apply(stream)
	}

	return stream
}

func TestConditionHolds(t *testing.T) {
	for _, tc := range []struct {
		name      string
		condition Condition
		picture   *MediaStream
		want      bool
	}{
		{
			name:      "an SDR picture where the client asked for SDR",
			condition: Condition{Property: "VideoRangeType", Verb: EqualsAny, Value: "SDR"},
			picture:   picture(func(s *MediaStream) { s.VideoRangeType = streammodal.VideoRangeTypeSDR }),
			want:      true,
		},
		{
			name:      "an HDR10 picture where the client asked for SDR",
			condition: Condition{Property: "VideoRangeType", Verb: EqualsAny, Value: "SDR"},
			picture:   picture(func(s *MediaStream) { s.VideoRangeType = streammodal.VideoRangeTypeHDR10 }),
			want:      false,
		},
		{
			name:      "an HLG picture against a client that lists two ranges",
			condition: Condition{Property: "VideoRangeType", Verb: EqualsAny, Value: "SDR|HLG"},
			picture:   picture(func(s *MediaStream) { s.VideoRangeType = streammodal.VideoRangeTypeHLG }),
			want:      true,
		},
		{
			name:      "a range nobody probed",
			condition: Condition{Property: "VideoRangeType", Verb: EqualsAny, Value: "SDR"},
			picture:   picture(nil),
			want:      true,
		},
		{
			name:      "a range the probe could not name",
			condition: Condition{Property: "VideoRangeType", Verb: EqualsAny, Value: "SDR"},
			picture:   picture(func(s *MediaStream) { s.VideoRangeType = streammodal.VideoRangeTypeUnknown }),
			want:      true,
		},
		{
			name:      "a profile the client knows",
			condition: Condition{Property: "VideoProfile", Verb: EqualsAny, Value: "high|main|baseline|constrained baseline"},
			picture:   picture(func(s *MediaStream) { s.Profile = "High" }),
			want:      true,
		},
		{
			name:      "ten bit video against a client that knows only eight",
			condition: Condition{Property: "VideoProfile", Verb: EqualsAny, Value: "high|main|baseline|constrained baseline"},
			picture:   picture(func(s *MediaStream) { s.Profile = "High 10" }),
			want:      false,
		},
		{
			name:      "a level the client can reach",
			condition: Condition{Property: "VideoLevel", Verb: LessThanEqual, Value: "52"},
			picture:   picture(func(s *MediaStream) { s.Level = 41 }),
			want:      true,
		},
		{
			name:      "a level past what the client can reach",
			condition: Condition{Property: "VideoLevel", Verb: LessThanEqual, Value: "52"},
			picture:   picture(func(s *MediaStream) { s.Level = 62 }),
			want:      false,
		},
		{
			name:      "a level nobody probed",
			condition: Condition{Property: "VideoLevel", Verb: LessThanEqual, Value: "52"},
			picture:   picture(nil),
			want:      true,
		},
		{
			name:      "a progressive picture where the client refuses interlaced",
			condition: Condition{Property: "IsInterlaced", Verb: NotEquals, Value: "true"},
			picture:   picture(nil),
			want:      true,
		},
		{
			name:      "an interlaced picture where the client refuses interlaced",
			condition: Condition{Property: "IsInterlaced", Verb: NotEquals, Value: "true"},
			picture:   picture(func(s *MediaStream) { s.IsInterlaced = true }),
			want:      false,
		},
		{
			name:      "an anamorphic picture where the client refuses one",
			condition: Condition{Property: "IsAnamorphic", Verb: NotEquals, Value: "true"},
			picture:   picture(func(s *MediaStream) { s.IsAnamorphic = true }),
			want:      false,
		},
		{
			name:      "a width the client can take",
			condition: Condition{Property: "Width", Verb: LessThanEqual, Value: "1920"},
			picture:   picture(func(s *MediaStream) { s.Width = 1920 }),
			want:      true,
		},
		{
			name:      "a width past what the client can take",
			condition: Condition{Property: "Width", Verb: LessThanEqual, Value: "1920"},
			picture:   picture(func(s *MediaStream) { s.Width = 3840 }),
			want:      false,
		},
		{
			name:      "a bitrate past what the client can take",
			condition: Condition{Property: "VideoBitrate", Verb: LessThanEqual, Value: "8000000"},
			picture:   picture(func(s *MediaStream) { s.BitRate = 20_000_000 }),
			want:      false,
		},
		{
			name:      "a verb nothing here has been tested against",
			condition: Condition{Property: "VideoProfile", Verb: "EqualsNone", Value: "high"},
			picture:   picture(func(s *MediaStream) { s.Profile = "High" }),
			want:      true,
		},
		{
			name:      "a property nothing here reads",
			condition: Condition{Property: "VideoFramerate", Verb: LessThanEqual, Value: "30"},
			picture:   picture(nil),
			want:      true,
		},
		{
			name:      "a ceiling that is not a number",
			condition: Condition{Property: "VideoLevel", Verb: LessThanEqual, Value: "high"},
			picture:   picture(func(s *MediaStream) { s.Level = 41 }),
			want:      true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.condition.holds(tc.picture); got != tc.want {
				t.Errorf("holds = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCapabilities_satisfies(t *testing.T) {
	t.Run("a source with no picture is not held to conditions about one", func(t *testing.T) {
		source := &MediaSource{Container: "flac"}
		source.Edges.Streams = []*MediaStream{{Index: 0, Kind: streammodal.KindAudio, Codec: "flac"}}

		if !chrome.satisfies(source) {
			t.Error("a song was refused for a condition about a picture it does not have")
		}
	})

	t.Run("conditions for another codec are not applied", func(t *testing.T) {
		can := Capabilities{Codecs: []CodecCondition{{
			Codec:      "hevc",
			Conditions: []Condition{{Property: "VideoRangeType", Verb: EqualsAny, Value: "SDR"}},
		}}}

		source := ripped("mkv", "h264", 2160, "aac")
		source.Edges.Streams[0].VideoRangeType = streammodal.VideoRangeTypeHDR10

		if !can.satisfies(source) {
			t.Error("an h264 picture was held to a condition the client wrote for hevc")
		}
	})

	t.Run("a condition naming no codec is applied to every picture", func(t *testing.T) {
		can := Capabilities{Codecs: []CodecCondition{{
			Conditions: []Condition{{Property: "Width", Verb: LessThanEqual, Value: "1920"}},
		}}}

		source := ripped("mkv", "h264", 2160, "aac")

		if can.satisfies(source) {
			t.Error("a 4K picture passed a width ceiling the client set for everything")
		}
	})
}
