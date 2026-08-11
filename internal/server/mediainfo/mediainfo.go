package mediainfo

import (
	"bytes"
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
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
		PlaySessionId: apiutil.Ptr(session),
	}, nil
}

func (s *Server) mediaSource(ctx context.Context, item *items.Item) (api.MediaSourceInfo, error) {
	source, err := s.items.MediaSource(ctx, item.ID)
	if err != nil {
		return api.MediaSourceInfo{}, err
	}
	if source == nil {
		return unprobedSource(item), nil
	}

	streams := source.Edges.Streams
	converted := make([]api.MediaStream, 0, len(streams))
	for _, stream := range streams {
		converted = append(converted, mediaStreamDto(stream))
	}

	return api.MediaSourceInfo{
		Id:                         apiutil.Ptr(source.ID.String()),
		Name:                       apiutil.Ptr(source.Name),
		Path:                       apiutil.Ptr(source.Path),
		Protocol:                   apiutil.Ptr(api.MediaProtocol(source.Protocol)),
		Type:                       apiutil.Ptr(api.MediaSourceType(source.Kind)),
		Container:                  apiutil.Ptr(source.Container),
		Size:                       apiutil.Ptr(source.Size),
		Bitrate:                    apiutil.Ptr(source.Bitrate),
		RunTimeTicks:               apiutil.Ptr(source.RunTimeTicks),
		MediaStreams:               &converted,
		MediaAttachments:           &[]api.MediaAttachment{},
		Formats:                    &[]string{},
		SupportsDirectPlay:         apiutil.Ptr(source.SupportsDirectPlay),
		SupportsDirectStream:       apiutil.Ptr(source.SupportsDirectStream),
		SupportsTranscoding:        apiutil.Ptr(source.SupportsTranscoding),
		SupportsProbing:            apiutil.Ptr(source.SupportsProbing),
		IsRemote:                   apiutil.Ptr(source.IsRemote),
		IsInfiniteStream:           apiutil.Ptr(source.IsInfiniteStream),
		RequiresOpening:            apiutil.Ptr(false),
		RequiresClosing:            apiutil.Ptr(false),
		RequiresLooping:            apiutil.Ptr(source.RequiresLooping),
		DefaultAudioStreamIndex:    defaultStreamIndex(streams, streammodal.KindAudio),
		DefaultSubtitleStreamIndex: defaultStreamIndex(streams, streammodal.KindSubtitle),
	}, nil
}

// An item the probe has not reached yet still has to answer with something
// playable, or the client refuses to start.
func unprobedSource(item *items.Item) api.MediaSourceInfo {
	return api.MediaSourceInfo{
		Id:                   apiutil.Ptr(item.ID.String()),
		Name:                 apiutil.Ptr(item.Name),
		Path:                 apiutil.Ptr(item.Path),
		Protocol:             apiutil.Ptr(api.MediaProtocolFile),
		Type:                 apiutil.Ptr(api.MediaSourceTypeDefault),
		Container:            apiutil.Ptr(item.Container),
		RunTimeTicks:         item.RunTimeTicks,
		MediaStreams:         &[]api.MediaStream{},
		MediaAttachments:     &[]api.MediaAttachment{},
		Formats:              &[]string{},
		SupportsDirectPlay:   apiutil.Ptr(true),
		SupportsDirectStream: apiutil.Ptr(true),
		SupportsTranscoding:  apiutil.Ptr(false),
		SupportsProbing:      apiutil.Ptr(true),
		IsRemote:             apiutil.Ptr(false),
		IsInfiniteStream:     apiutil.Ptr(false),
		RequiresOpening:      apiutil.Ptr(false),
		RequiresClosing:      apiutil.Ptr(false),
		RequiresLooping:      apiutil.Ptr(false),
	}
}

func mediaStreamDto(stream *items.MediaStream) api.MediaStream {
	kind := api.MediaStreamType(stream.Kind)

	dto := api.MediaStream{
		Index:                  apiutil.Ptr(stream.Index),
		Type:                   &kind,
		Codec:                  apiutil.Ptr(stream.Codec),
		IsDefault:              apiutil.Ptr(stream.IsDefault),
		IsForced:               apiutil.Ptr(stream.IsForced),
		IsExternal:             apiutil.Ptr(false),
		IsInterlaced:           apiutil.Ptr(false),
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
	if stream.BitRate > 0 {
		dto.BitRate = apiutil.Ptr(stream.BitRate)
	}
	if stream.PixelFormat != "" {
		dto.PixelFormat = apiutil.Ptr(stream.PixelFormat)
	}
	if stream.Level > 0 {
		dto.Level = apiutil.Ptr(stream.Level)
	}

	switch stream.Kind {
	case streammodal.KindVideo:
		dto.Width = apiutil.Ptr(stream.Width)
		dto.Height = apiutil.Ptr(stream.Height)
		dto.AspectRatio = apiutil.Ptr(aspectRatio(stream.Width, stream.Height))
	case streammodal.KindAudio:
		dto.Channels = apiutil.Ptr(stream.Channels)
		dto.SampleRate = apiutil.Ptr(stream.SampleRate)
	}

	return dto
}

func streamDisplayTitle(stream *items.MediaStream) string {
	switch stream.Kind {
	case streammodal.KindVideo:
		return fmt.Sprintf("%dx%d %s", stream.Width, stream.Height, stream.Codec)
	case streammodal.KindAudio:
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
