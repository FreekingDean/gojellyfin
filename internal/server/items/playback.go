package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

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
	item, err := s.ItemByID(ctx, itemID)
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
		PlaySessionId: ptr(session),
	}, nil
}

func (s *Server) mediaSource(ctx context.Context, item *Item) (api.MediaSourceInfo, error) {
	streams, err := s.ListMediaStreams(ctx, item.ID)
	if err != nil {
		return api.MediaSourceInfo{}, err
	}

	dtos := make([]api.MediaStream, 0, len(streams))
	for _, stream := range streams {
		dtos = append(dtos, mediaStreamDto(&stream))
	}

	return api.MediaSourceInfo{
		Id:                         ptr(item.ID.String()),
		Name:                       ptr(item.Name),
		Path:                       ptr(item.Path),
		Protocol:                   ptr(api.MediaProtocolFile),
		Type:                       ptr(api.MediaSourceTypeDefault),
		Container:                  ptr(item.Container),
		Size:                       ptr(item.Size),
		Bitrate:                    ptr(item.Bitrate),
		RunTimeTicks:               item.RunTimeTicks,
		MediaStreams:               &dtos,
		MediaAttachments:           &[]api.MediaAttachment{},
		Formats:                    &[]string{},
		SupportsDirectPlay:         ptr(true),
		SupportsDirectStream:       ptr(true),
		SupportsTranscoding:        ptr(false),
		SupportsProbing:            ptr(true),
		IsRemote:                   ptr(false),
		IsInfiniteStream:           ptr(false),
		RequiresOpening:            ptr(false),
		RequiresClosing:            ptr(false),
		RequiresLooping:            ptr(false),
		DefaultAudioStreamIndex:    defaultStreamIndex(streams, "Audio"),
		DefaultSubtitleStreamIndex: defaultStreamIndex(streams, "Subtitle"),
	}, nil
}

func mediaStreamDto(stream *MediaStream) api.MediaStream {
	kind := api.MediaStreamType(stream.Type)

	dto := api.MediaStream{
		Index:                  ptr(stream.Index),
		Type:                   &kind,
		Codec:                  ptr(stream.Codec),
		IsDefault:              ptr(stream.IsDefault),
		IsForced:               ptr(stream.IsForced),
		IsExternal:             ptr(false),
		IsInterlaced:           ptr(false),
		SupportsExternalStream: ptr(false),
		DisplayTitle:           ptr(streamDisplayTitle(stream)),
	}

	if stream.Profile != "" {
		dto.Profile = ptr(stream.Profile)
	}
	if stream.Language != "" {
		dto.Language = ptr(stream.Language)
	}
	if stream.Title != "" {
		dto.Title = ptr(stream.Title)
	}
	if stream.Bitrate > 0 {
		dto.BitRate = ptr(stream.Bitrate)
	}
	if stream.PixelFormat != "" {
		dto.PixelFormat = ptr(stream.PixelFormat)
	}
	if stream.Level > 0 {
		dto.Level = ptr(float64(stream.Level))
	}

	switch stream.Type {
	case "Video":
		dto.Width = ptr(stream.Width)
		dto.Height = ptr(stream.Height)
		dto.AspectRatio = ptr(aspectRatio(stream.Width, stream.Height))
	case "Audio":
		dto.Channels = ptr(stream.Channels)
		dto.SampleRate = ptr(stream.SampleRate)
	}

	return dto
}

func streamDisplayTitle(stream *MediaStream) string {
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

func defaultStreamIndex(streams []MediaStream, kind string) *int32 {
	for _, stream := range streams {
		if stream.Type == kind && stream.IsDefault {
			return ptr(stream.Index)
		}
	}
	for _, stream := range streams {
		if stream.Type == kind {
			return ptr(stream.Index)
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
