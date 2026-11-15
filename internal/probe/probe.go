package probe

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

const (
	ticksPerSecond = 10_000_000
	maxLyricsBytes = 1 << 20
	heartbeatEvery = 30 * time.Second
)

func (s *Prober) probeFile(ctx context.Context, source *items.MediaSource) (*items.MediaSource, error) {
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

	streams := make([]*items.MediaStream, 0, len(probe.Streams))
	for _, stream := range probe.Streams {
		streams = append(streams, &items.MediaStream{
			Index:       stream.Index,
			Kind:        streamKind(stream.CodecType),
			Codec:       stream.CodecName,
			Profile:     stream.Profile,
			Language:    stream.Tags["language"],
			Title:       stream.Tags["title"],
			Width:       optional(stream.Width),
			Height:      optional(stream.Height),
			Channels:    optional(stream.Channels),
			SampleRate:  optional(stream.SampleRate),
			BitRate:     optional(stream.BitRate),
			PixelFormat: stream.PixelFormat,
			Level:       optional(stream.Level),
			IsDefault:   stream.Disposition.Default,
			IsForced:    stream.Disposition.Forced,

			VideoRangeType: optional(rangeType(stream.ColorTransfer)),
			IsInterlaced:   interlaced(stream.FieldOrder),
			IsAnamorphic:   anamorphic(stream.AspectRatio),
		})
	}

	keyframes, err := s.keyframes(ctx, source.Path, probe)
	if err != nil {
		return nil, err
	}

	return &items.MediaSource{
		Keyframes:    keyframes,
		Container:    container(probe.Format.FormatName, source.Path),
		RunTimeTicks: optional(int64(probe.Format.Duration * ticksPerSecond)),
		Size:         optional(probe.Format.Size),
		Bitrate:      optional(probe.Format.BitRate),
		Streams:      streams,
		Segments:     chapterSegments(probe.Chapters),
		Lyrics:       lyricsOf(source.Path, probe),
	}, nil
}

func (s *Prober) keyframes(ctx context.Context, path string, probe *ffmpeg.Probe) (*items.KeyframeIndex, error) {
	if !slices.ContainsFunc(probe.Streams, func(stream ffmpeg.Stream) bool { return stream.CodecType == "video" }) {
		return nil, nil
	}

	done := make(chan struct{})
	defer close(done)
	go beat(ctx, done, path)

	ticks, err := s.ffmpeg.Keyframes(ctx, path)
	if err != nil {
		return nil, err
	}

	return &items.KeyframeIndex{Ticks: ticks}, nil
}

func beat(ctx context.Context, done <-chan struct{}, path string) {
	ticker := time.NewTicker(heartbeatEvery)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobs.Heartbeat(ctx, path)
		}
	}
}

func lyricsOf(path string, probe *ffmpeg.Probe) string {
	for _, tags := range tagSets(probe) {
		for name, value := range tags {
			lowered := strings.ToLower(name)
			if (lowered == "lyrics" || lowered == "unsyncedlyrics" || strings.HasPrefix(lowered, "lyrics-")) && strings.TrimSpace(value) != "" {
				return value
			}
		}
	}

	sidecar, err := os.Open(strings.TrimSuffix(path, filepath.Ext(path)) + ".lrc")
	if err != nil {
		return ""
	}
	defer func() { _ = sidecar.Close() }()

	text, err := io.ReadAll(io.LimitReader(sidecar, maxLyricsBytes))
	if err != nil || !utf8.Valid(text) {
		return ""
	}

	return string(text)
}

func tagSets(probe *ffmpeg.Probe) []map[string]string {
	sets := []map[string]string{probe.Format.Tags}
	for _, stream := range probe.Streams {
		sets = append(sets, stream.Tags)
	}

	return sets
}

func rangeType(transfer string) items.VideoRangeType {
	switch strings.ToLower(strings.TrimSpace(transfer)) {
	case "smpte2084":
		return streammodel.VideoRangeTypeHDR10
	case "arib-std-b67":
		return streammodel.VideoRangeTypeHLG
	case "":
		return ""
	default:
		return streammodel.VideoRangeTypeSDR
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
		return streammodel.KindVideo
	case "audio":
		return streammodel.KindAudio
	case "subtitle":
		return streammodel.KindSubtitle
	default:
		return streammodel.KindEmbeddedImage
	}
}

func optional[T comparable](value T) *T {
	var zero T
	if value == zero {
		return nil
	}

	return &value
}
