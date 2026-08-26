package mediainfo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
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
	response, err := s.playbackInfo(ctx, request.ItemId, nil, 0)
	if err != nil {
		return nil, err
	}

	return api.GetPlaybackInfo200JSONResponse(response), nil
}

func (s *Server) GetPostedPlaybackInfo(ctx context.Context, request api.GetPostedPlaybackInfoRequestObject) (api.GetPostedPlaybackInfoResponseObject, error) {
	var profile api.DeviceProfile
	startTicks := apiutil.Deref(request.Params.StartTimeTicks)
	if body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody); body != nil {
		if body.DeviceProfile != nil {
			profile = *body.DeviceProfile
		}
		if body.StartTimeTicks != nil {
			startTicks = *body.StartTimeTicks
		}
	}

	response, err := s.playbackInfo(ctx, request.ItemId, profile, startTicks)
	if err != nil {
		return nil, err
	}

	return api.GetPostedPlaybackInfo200JSONResponse(response), nil
}

func (s *Server) playbackInfo(ctx context.Context, itemID uuid.UUID, profile api.DeviceProfile, startTicks int64) (api.PlaybackInfoResponse, error) {
	item, err := s.items.ItemByID(ctx, itemID)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	session, err := newPlaySessionId()
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	response := api.PlaybackInfoResponse{
		MediaSources:  &[]api.MediaSourceInfo{},
		PlaySessionId: apiutil.Ptr(session),
	}

	plan, err := s.items.SourceFor(ctx, itemID, capabilities(profile))
	if errors.Is(err, items.ErrNoSource) || errors.Is(err, items.ErrNoPlayable) {
		log.Printf("nothing to play for %s: %v", item.Name, err)
		response.ErrorCode = apiutil.Ptr(api.NoCompatibleStream)

		return response, nil
	}
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	dto := mediaSourceDto(plan.Source)
	if !items.IsAudio(item) {
		served(&dto, plan, streamURL(itemID, plan, auth.AuthorizationFrom(ctx).Token, session, startTicks))
	}

	response.MediaSources = &[]api.MediaSourceInfo{dto}

	return response, nil
}

func served(dto *api.MediaSourceInfo, plan items.Plan, url string) {
	dto.SupportsDirectPlay = apiutil.Ptr(false)
	dto.SupportsDirectStream = apiutil.Ptr(false)
	dto.SupportsTranscoding = apiutil.Ptr(true)
	dto.TranscodingUrl = apiutil.Ptr(url)
	dto.TranscodingContainer = apiutil.Ptr(plan.Container)
	dto.TranscodingSubProtocol = apiutil.Ptr(api.MediaStreamProtocolHttp)
}

func mediaSourceDto(source *items.MediaSource) api.MediaSourceInfo {
	streams := source.Edges.Streams
	converted := make([]api.MediaStream, 0, len(streams))
	for _, stream := range streams {
		converted = append(converted, mediaStreamDto(source, stream))
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
		SupportsTranscoding:        apiutil.Ptr(false),
		SupportsProbing:            apiutil.Ptr(source.SupportsProbing),
		IsRemote:                   apiutil.Ptr(source.IsRemote),
		IsInfiniteStream:           apiutil.Ptr(source.IsInfiniteStream),
		RequiresOpening:            apiutil.Ptr(false),
		RequiresClosing:            apiutil.Ptr(false),
		RequiresLooping:            apiutil.Ptr(source.RequiresLooping),
		DefaultAudioStreamIndex:    defaultStreamIndex(streams, streammodal.KindAudio),
		DefaultSubtitleStreamIndex: defaultStreamIndex(streams, streammodal.KindSubtitle),
	}
}

func mediaStreamDto(source *items.MediaSource, stream *items.MediaStream) api.MediaStream {
	kind := api.MediaStreamType(stream.Kind)

	dto := api.MediaStream{
		Index:                  apiutil.Ptr(stream.Index),
		Type:                   &kind,
		Codec:                  apiutil.Ptr(stream.Codec),
		IsDefault:              apiutil.Ptr(stream.IsDefault),
		IsForced:               apiutil.Ptr(stream.IsForced),
		IsExternal:             apiutil.Ptr(stream.IsExternal),
		IsInterlaced:           apiutil.Ptr(false),
		SupportsExternalStream: apiutil.Ptr(stream.IsExternal),
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
	case streammodal.KindSubtitle:
		dto.IsHearingImpaired = apiutil.Ptr(stream.IsHearingImpaired)
		if stream.IsExternal {
			dto.Path = apiutil.Ptr(stream.Path)
			dto.IsTextSubtitleStream = apiutil.Ptr(true)
			dto.DeliveryMethod = apiutil.Ptr(api.SubtitleDeliveryMethodExternal)
			dto.DeliveryUrl = apiutil.Ptr(subtitleURL(source, stream))
		}
	}

	return dto
}

func subtitleURL(source *items.MediaSource, stream *items.MediaStream) string {
	return fmt.Sprintf("/Videos/%s/%s/Subtitles/%d/0/Stream.vtt", source.ItemID, source.ID, stream.Index)
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

const (
	defaultBitrateTestSize = 102400
	maxBitrateTestSize     = 100_000_000
)

type zeroes struct{}

func (zeroes) Read(p []byte) (int, error) {
	clear(p)

	return len(p), nil
}

func (s *Server) GetBitrateTestBytes(ctx context.Context, request api.GetBitrateTestBytesRequestObject) (api.GetBitrateTestBytesResponseObject, error) {
	size := int64(apiutil.OrElse(request.Params.Size, int32(defaultBitrateTestSize)))
	size = min(max(size, 1), maxBitrateTestSize)

	return api.GetBitrateTestBytes200ApplicationoctetStreamResponse{
		Body:          io.LimitReader(zeroes{}, size),
		ContentLength: size,
	}, nil
}
