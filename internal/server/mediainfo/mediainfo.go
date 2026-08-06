package mediainfo

import (
	"bytes"
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetPlaybackInfo(ctx context.Context, request api.GetPlaybackInfoRequestObject) (api.GetPlaybackInfoResponseObject, error) {
	response, err := s.playbackInfo(ctx, request.ItemId)
	if err != nil {
		return nil, err
	}

	return api.GetPlaybackInfo200JSONResponse(response), nil
}

func (s *Server) GetPostedPlaybackInfo(ctx context.Context, request api.GetPostedPlaybackInfoRequestObject) (api.GetPostedPlaybackInfoResponseObject, error) {
	response, err := s.playbackInfo(ctx, request.ItemId)
	if err != nil {
		return nil, err
	}

	return api.GetPostedPlaybackInfo200JSONResponse(response), nil
}

func (s *Server) playbackInfo(ctx context.Context, itemID uuid.UUID) (api.PlaybackInfoResponse, error) {
	item, err := s.items.ItemByID(ctx, itemID)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	source, err := s.mediaSource(ctx, item)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	session, err := newPlaySessionId()
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	return api.PlaybackInfoResponse{
		MediaSources:  &[]api.MediaSourceInfo{source},
		PlaySessionId: dtos.Ptr(session),
	}, nil
}

func (s *Server) mediaSource(ctx context.Context, item *items.Item) (api.MediaSourceInfo, error) {
	streams, err := s.items.ListMediaStreams(ctx, item.ID)
	if err != nil {
		return api.MediaSourceInfo{}, err
	}

	converted := make([]api.MediaStream, 0, len(streams))
	for _, stream := range streams {
		converted = append(converted, mediaStreamDto(&stream))
	}

	return api.MediaSourceInfo{
		Id:                         dtos.Ptr(item.ID.String()),
		Name:                       dtos.Ptr(item.Name),
		Path:                       dtos.Ptr(item.Path),
		Protocol:                   dtos.Ptr(api.MediaProtocolFile),
		Type:                       dtos.Ptr(api.MediaSourceTypeDefault),
		Container:                  dtos.Ptr(item.Container),
		Size:                       dtos.Ptr(item.Size),
		Bitrate:                    dtos.Ptr(item.Bitrate),
		RunTimeTicks:               item.RunTimeTicks,
		MediaStreams:               &converted,
		MediaAttachments:           &[]api.MediaAttachment{},
		Formats:                    &[]string{},
		SupportsDirectPlay:         dtos.Ptr(true),
		SupportsDirectStream:       dtos.Ptr(true),
		SupportsTranscoding:        dtos.Ptr(false),
		SupportsProbing:            dtos.Ptr(true),
		IsRemote:                   dtos.Ptr(false),
		IsInfiniteStream:           dtos.Ptr(false),
		RequiresOpening:            dtos.Ptr(false),
		RequiresClosing:            dtos.Ptr(false),
		RequiresLooping:            dtos.Ptr(false),
		DefaultAudioStreamIndex:    defaultStreamIndex(streams, "Audio"),
		DefaultSubtitleStreamIndex: defaultStreamIndex(streams, "Subtitle"),
	}, nil
}

func mediaStreamDto(stream *items.MediaStream) api.MediaStream {
	kind := api.MediaStreamType(stream.Type)

	dto := api.MediaStream{
		Index:                  dtos.Ptr(stream.Index),
		Type:                   &kind,
		Codec:                  dtos.Ptr(stream.Codec),
		IsDefault:              dtos.Ptr(stream.IsDefault),
		IsForced:               dtos.Ptr(stream.IsForced),
		IsExternal:             dtos.Ptr(false),
		IsInterlaced:           dtos.Ptr(false),
		SupportsExternalStream: dtos.Ptr(false),
		DisplayTitle:           dtos.Ptr(streamDisplayTitle(stream)),
	}

	if stream.Profile != "" {
		dto.Profile = dtos.Ptr(stream.Profile)
	}
	if stream.Language != "" {
		dto.Language = dtos.Ptr(stream.Language)
	}
	if stream.Title != "" {
		dto.Title = dtos.Ptr(stream.Title)
	}
	if stream.Bitrate > 0 {
		dto.BitRate = dtos.Ptr(stream.Bitrate)
	}
	if stream.PixelFormat != "" {
		dto.PixelFormat = dtos.Ptr(stream.PixelFormat)
	}
	if stream.Level > 0 {
		dto.Level = dtos.Ptr(float64(stream.Level))
	}

	switch stream.Type {
	case "Video":
		dto.Width = dtos.Ptr(stream.Width)
		dto.Height = dtos.Ptr(stream.Height)
		dto.AspectRatio = dtos.Ptr(aspectRatio(stream.Width, stream.Height))
	case "Audio":
		dto.Channels = dtos.Ptr(stream.Channels)
		dto.SampleRate = dtos.Ptr(stream.SampleRate)
	}

	return dto
}

func streamDisplayTitle(stream *items.MediaStream) string {
	switch stream.Type {
	case "Video":
		return fmt.Sprintf("%dx%d %s", stream.Width, stream.Height, stream.Codec)
	case "Audio":
		if stream.Language != "" {
			return fmt.Sprintf("%s %s", stream.Language, stream.Codec)
		}

		return stream.Codec
	default:
		if stream.Title != "" {
			return stream.Title
		}

		return stream.Codec
	}
}

func defaultStreamIndex(streams []items.MediaStream, kind string) *int32 {
	for _, stream := range streams {
		if stream.Type == kind && stream.IsDefault {
			return dtos.Ptr(stream.Index)
		}
	}
	for _, stream := range streams {
		if stream.Type == kind {
			return dtos.Ptr(stream.Index)
		}
	}

	return nil
}

func aspectRatio(width, height int32) string {
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

func newPlaySessionId() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return id.String(), nil
}

func (s *Server) GetBitrateTestBytes(ctx context.Context, request api.GetBitrateTestBytesRequestObject) (api.GetBitrateTestBytesResponseObject, error) {
	buf := bytes.NewBufferString("This is a test endpoint info response")
	return api.GetBitrateTestBytes200ApplicationoctetStreamResponse{
		Body:          buf,
		ContentLength: int64(buf.Len()),
	}, nil
}

func (s *Server) GetEndpointInfo(ctx context.Context, request api.GetEndpointInfoRequestObject) (api.GetEndpointInfoResponseObject, error) {
	return api.GetEndpointInfo200JSONResponse{}, nil
}
