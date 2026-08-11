package scanner

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

const ticksPerSecond = 10_000_000

func (s *Scanner) probe(ctx context.Context, item *items.Item) error {
	if !items.NeedsProbe(item) {
		return nil
	}

	probe, err := ffmpeg.ProbeFile(ctx, item.Path)
	if err != nil {
		return err
	}

	streams := make([]items.Stream, 0, len(probe.Streams))
	for _, stream := range probe.Streams {
		streams = append(streams, items.Stream{
			Index:       int32(stream.Index),
			Kind:        streamKind(stream.CodecType),
			Codec:       stream.CodecName,
			Profile:     stream.Profile,
			Language:    stream.Language(),
			Title:       stream.Title(),
			Width:       int32(stream.Width),
			Height:      int32(stream.Height),
			Channels:    int32(stream.Channels),
			SampleRate:  stream.SampleRateHz(),
			Bitrate:     stream.Bitrate(),
			PixelFormat: stream.PixelFormat,
			Level:       float64(stream.Level),
			IsDefault:   stream.IsDefault(),
			IsForced:    stream.IsForced(),
		})
	}

	return s.items.SaveProbe(ctx, item, items.Probe{
		Container:    container(probe.Format.FormatName, item.Path),
		RunTimeTicks: int64(probe.Format.Seconds() * ticksPerSecond),
		Size:         probe.Format.Bytes(),
		Bitrate:      probe.Format.Bitrate(),
		Streams:      streams,
	})
}

// ffprobe reports muxer families like "matroska,webm"; the file extension picks
// the one clients should be told about.
func container(formatName, path string) string {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	for _, name := range strings.Split(formatName, ",") {
		if name == extension {
			return name
		}
	}

	return extension
}

func streamKind(codecType string) items.StreamKind {
	switch codecType {
	case "video":
		return streammodal.KindVideo
	case "audio":
		return streammodal.KindAudio
	case "subtitle":
		return streammodal.KindSubtitle
	default:
		return streammodal.KindEmbeddedImage
	}
}
