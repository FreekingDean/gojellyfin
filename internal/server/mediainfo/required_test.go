package mediainfo

import (
	"encoding/json"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/items"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

func TestMediaSourceDto_CarriesWhatTheKotlinSDKRequires(t *testing.T) {
	source := &items.MediaSource{Streams: []*items.MediaStream{
		{Kind: streammodel.KindSubtitle, Codec: "subrip"},
		{Kind: streammodel.KindSubtitle, Codec: "hdmv_pgs_subtitle"},
	}}

	encoded, err := json.Marshal(mediaSourceDto(source))
	if err != nil {
		t.Fatalf("failed to encode the source: %v", err)
	}

	var decoded struct {
		MediaStreams []map[string]any
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("failed to decode the source: %v", err)
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to decode the streams: %v", err)
	}

	for _, required := range []string{
		"Protocol", "Type", "IsRemote", "ReadAtNativeFramerate", "IgnoreDts", "IgnoreIndex", "GenPtsInput",
		"SupportsTranscoding", "SupportsDirectStream", "SupportsDirectPlay", "IsInfiniteStream",
		"RequiresOpening", "RequiresClosing", "RequiresLooping", "SupportsProbing", "HasSegments",
	} {
		if _, ok := fields[required]; !ok {
			t.Errorf("MediaSourceInfo carries no %s, which the Kotlin SDK refuses to decode", required)
		}
	}

	for _, stream := range decoded.MediaStreams {
		for _, required := range []string{
			"IsInterlaced", "IsDefault", "IsForced", "IsHearingImpaired", "Type", "Index",
			"IsExternal", "IsTextSubtitleStream", "SupportsExternalStream",
		} {
			if _, ok := stream[required]; !ok {
				t.Errorf("MediaStream carries no %s, which the Kotlin SDK refuses to decode", required)
			}
		}
	}

	if decoded.MediaStreams[0]["IsTextSubtitleStream"] != true {
		t.Errorf("a subrip stream is text, got %v", decoded.MediaStreams[0]["IsTextSubtitleStream"])
	}
	if decoded.MediaStreams[1]["IsTextSubtitleStream"] != false {
		t.Errorf("a PGS stream is a picture, got %v", decoded.MediaStreams[1]["IsTextSubtitleStream"])
	}
}
