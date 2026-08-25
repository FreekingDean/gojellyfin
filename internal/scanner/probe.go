package scanner

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

const ticksPerSecond = 10_000_000

func (s *Scanner) probeFile(ctx context.Context, source *items.MediaSource) (*items.Probe, error) {
	if !items.NeedsProbe(source) {
		return nil, nil
	}

	probe, err := s.ffmpeg.ProbeFile(ctx, source.Path)
	if errors.Is(err, ffmpeg.ErrNotAvailable) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	streams := make([]items.Stream, 0, len(probe.Streams))
	for _, stream := range probe.Streams {
		streams = append(streams, items.Stream{
			Index:       stream.Index,
			Kind:        streamKind(stream.CodecType),
			Codec:       stream.CodecName,
			Profile:     stream.Profile,
			Language:    stream.Tags["language"],
			Title:       stream.Tags["title"],
			Width:       stream.Width,
			Height:      stream.Height,
			Channels:    stream.Channels,
			SampleRate:  stream.SampleRate,
			Bitrate:     stream.BitRate,
			PixelFormat: stream.PixelFormat,
			Level:       stream.Level,
			IsDefault:   stream.Disposition.Default,
			IsForced:    stream.Disposition.Forced,

			RangeType:    rangeType(stream.ColorTransfer),
			IsInterlaced: interlaced(stream.FieldOrder),
			IsAnamorphic: anamorphic(stream.AspectRatio),
		})
	}

	return &items.Probe{
		Container:    container(probe.Format.FormatName, source.Path),
		RunTimeTicks: int64(probe.Format.Duration * ticksPerSecond),
		Size:         probe.Format.Size,
		Bitrate:      probe.Format.BitRate,
		Streams:      streams,
		Metadata:     metadata(probe),
	}, nil
}

func rangeType(transfer string) items.VideoRangeType {
	switch strings.ToLower(strings.TrimSpace(transfer)) {
	case "smpte2084":
		return streammodal.VideoRangeTypeHDR10
	case "arib-std-b67":
		return streammodal.VideoRangeTypeHLG
	case "":
		return ""
	default:
		return streammodal.VideoRangeTypeSDR
	}
}

func interlaced(order string) bool {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "tt", "bb", "tb", "bt":
		return true
	default:
		return false
	}
}

func anamorphic(ratio string) bool {
	switch ratio = strings.TrimSpace(ratio); ratio {
	case "", "0:1", "1:1", "N/A":
		return false
	default:
		return true
	}
}

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
