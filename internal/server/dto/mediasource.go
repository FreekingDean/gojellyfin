package dto

import (
	"fmt"
	"slices"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

func MediaSourceDto(source *items.MediaSource) api.MediaSourceInfo {
	streams := source.Streams
	converted := make([]api.MediaStream, 0, len(streams))
	for _, stream := range streams {
		converted = append(converted, mediaStreamDto(stream))
	}

	return api.MediaSourceInfo{
		Id:                         apiutil.Ptr(source.ID.String()),
		Name:                       apiutil.Ptr(source.Name),
		Protocol:                   apiutil.Ptr(api.MediaProtocolFile),
		Type:                       apiutil.Ptr(api.MediaSourceTypeDefault),
		Container:                  apiutil.Ptr(source.Container),
		Size:                       source.Size,
		Bitrate:                    source.Bitrate,
		RunTimeTicks:               source.RunTimeTicks,
		MediaStreams:               &converted,
		MediaAttachments:           &[]api.MediaAttachment{},
		Formats:                    &[]string{},
		SupportsDirectPlay:         apiutil.Ptr(true),
		SupportsDirectStream:       apiutil.Ptr(true),
		SupportsTranscoding:        apiutil.Ptr(false),
		SupportsProbing:            apiutil.Ptr(false),
		TranscodingSubProtocol:     apiutil.Ptr(api.MediaStreamProtocolHttp),
		IsRemote:                   apiutil.Ptr(false),
		IsInfiniteStream:           apiutil.Ptr(false),
		RequiresOpening:            apiutil.Ptr(false),
		RequiresClosing:            apiutil.Ptr(false),
		RequiresLooping:            apiutil.Ptr(false),
		ReadAtNativeFramerate:      apiutil.Ptr(false),
		IgnoreDts:                  apiutil.Ptr(false),
		IgnoreIndex:                apiutil.Ptr(false),
		GenPtsInput:                apiutil.Ptr(false),
		HasSegments:                apiutil.Ptr(len(source.Segments) > 0),
		DefaultAudioStreamIndex:    defaultStreamIndex(streams, streammodel.KindAudio),
		DefaultSubtitleStreamIndex: defaultStreamIndex(streams, streammodel.KindSubtitle),
	}
}

var pictureSubtitles = []string{"hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle", "xsub"}

func mediaStreamDto(stream *items.MediaStream) api.MediaStream {
	kind := api.MediaStreamType(stream.Kind)

	dto := api.MediaStream{
		Index:                  apiutil.Ptr(stream.Index),
		Type:                   &kind,
		Codec:                  apiutil.Ptr(stream.Codec),
		BitRate:                stream.BitRate,
		Level:                  stream.Level,
		IsDefault:              apiutil.Ptr(stream.IsDefault),
		IsForced:               apiutil.Ptr(stream.IsForced),
		IsExternal:             apiutil.Ptr(false),
		IsInterlaced:           apiutil.Ptr(false),
		IsHearingImpaired:      apiutil.Ptr(false),
		IsTextSubtitleStream:   apiutil.Ptr(stream.Kind == streammodel.KindSubtitle && !slices.Contains(pictureSubtitles, stream.Codec)),
		SupportsExternalStream: apiutil.Ptr(false),
		DisplayTitle:           apiutil.Ptr(streamDisplayTitle(stream)),
	}

	if stream.Profile != "" {
		dto.Profile = apiutil.Ptr(stream.Profile)
	}
	if stream.Language != "" {
		dto.Language = apiutil.Ptr(stream.Language)
	}
	if stream.Title != "" {
		dto.Title = apiutil.Ptr(stream.Title)
	}
	if stream.PixelFormat != "" {
		dto.PixelFormat = apiutil.Ptr(stream.PixelFormat)
	}

	switch stream.Kind {
	case streammodel.KindVideo:
		dto.Width = stream.Width
		dto.Height = stream.Height
		dto.AspectRatio = apiutil.Ptr(aspectRatio(stream))
	case streammodel.KindAudio:
		dto.Channels = stream.Channels
		dto.SampleRate = stream.SampleRate
	}

	return dto
}

func streamDisplayTitle(stream *items.MediaStream) string {
	switch stream.Kind {
	case streammodel.KindVideo:
		return fmt.Sprintf("%dx%d %s", apiutil.Deref(stream.Width), apiutil.Deref(stream.Height), stream.Codec)
	case streammodel.KindAudio:
		if stream.Language != "" {
			return fmt.Sprintf("%s %s", stream.Language, stream.Codec)
		}

		return stream.Codec
	default:
		if stream.Title != "" {
			return stream.Title
		}
		if stream.Language != "" {
			return stream.Language
		}

		return stream.Codec
	}
}

func defaultStreamIndex(streams []*items.MediaStream, kind items.StreamKind) *int32 {
	for _, stream := range streams {
		if stream.Kind == kind && stream.IsDefault {
			return apiutil.Ptr(stream.Index)
		}
	}
	for _, stream := range streams {
		if stream.Kind == kind {
			return apiutil.Ptr(stream.Index)
		}
	}

	return nil
}

func aspectRatio(stream *items.MediaStream) string {
	width, height := apiutil.Deref(stream.Width), apiutil.Deref(stream.Height)
	if width == 0 || height == 0 {
		return ""
	}

	divisor := gcd(width, height)

	return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
}

func gcd(a, b int32) int32 {
	for b != 0 {
		a, b = b, a%b
	}

	return a
}
