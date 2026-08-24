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

// Nil when there is nothing new to learn: no ffmpeg on the box, or a file the
// last probe already read.
func (s *Scanner) probeFile(ctx context.Context, source *items.MediaSource) (*items.Probe, error) {
	if !ffmpeg.Available() || !items.NeedsProbe(source) {
		return nil, nil
	}

	probe, err := ffmpeg.ProbeFile(ctx, source.Path)
	if err != nil {
		return nil, err
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

			RangeType:    rangeType(stream.ColorTransfer),
			IsInterlaced: interlaced(stream.FieldOrder),
			IsAnamorphic: anamorphic(stream.AspectRatio),
		})
	}

	return &items.Probe{
		Container:    container(probe.Format.FormatName, source.Path),
		RunTimeTicks: int64(probe.Format.Seconds() * ticksPerSecond),
		Size:         probe.Format.Bytes(),
		Bitrate:      probe.Format.Bitrate(),
		Streams:      streams,
		Metadata:     metadata(probe),
	}, nil
}

// The transfer characteristic is what separates an HDR picture from an SDR one,
// and it is the property a client's profile names. A file that carries none is
// left unknown rather than called SDR, because a guess here is answered by
// playing HDR through an SDR decoder, which does not fail loudly.
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

// ffprobe names the field order of a progressive picture, and one of tt, bb, tb
// or bt for the interlaced orders. Anything else is a picture it could not read
// the order of, which is not the same as an interlaced one.
func interlaced(order string) bool {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "tt", "bb", "tb", "bt":
		return true
	default:
		return false
	}
}

// Square pixels are written 1:1, and a file whose sample aspect ratio ffprobe
// could not read is not an anamorphic one.
func anamorphic(ratio string) bool {
	switch ratio = strings.TrimSpace(ratio); ratio {
	case "", "0:1", "1:1", "N/A":
		return false
	default:
		return true
	}
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
