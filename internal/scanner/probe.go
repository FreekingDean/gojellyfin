package scanner

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/server/items"
)

const ticksPerSecond = 10_000_000

func (s *Scanner) probe(ctx context.Context, libraryID uuid.UUID, path string) error {
	item, err := s.items.GetItemByPath(ctx, libraryID, path)
	if err != nil {
		return err
	}
	if item.ProbedAt != nil && !item.ProbedAt.Before(item.DateModified) {
		return nil
	}

	probe, err := ffmpeg.ProbeFile(ctx, path)
	if err != nil {
		return err
	}

	item.RunTimeTicks = ptr(int64(probe.Format.Seconds() * ticksPerSecond))
	item.Container = container(probe.Format.FormatName, path)
	item.Size = probe.Format.Bytes()
	item.Bitrate = probe.Format.Bitrate()

	if err := s.items.SaveItemMedia(ctx, item); err != nil {
		return err
	}

	streams := make([]items.MediaStream, 0, len(probe.Streams))
	for _, stream := range probe.Streams {
		streams = append(streams, items.MediaStream{
			ItemID:      item.ID,
			Index:       int32(stream.Index),
			Type:        streamType(stream.CodecType),
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
			Level:       int32(stream.Level),
			IsDefault:   stream.IsDefault(),
			IsForced:    stream.IsForced(),
		})
	}

	return s.items.ReplaceMediaStreams(ctx, item.ID, streams)
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

func streamType(codecType string) string {
	switch codecType {
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	case "subtitle":
		return "Subtitle"
	default:
		return "EmbeddedImage"
	}
}
