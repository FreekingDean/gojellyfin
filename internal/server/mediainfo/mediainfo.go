package mediainfo

import (
	"context"
	"errors"
	"io"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/prank"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

type Pranks interface {
	Swap(ctx context.Context, userID uuid.UUID, item *items.Item) (*items.Item, bool, error)
}

type Server struct {
	channels Channels
	policies dto.Access
	items    *items.Service
	sessions *sessions.Service
	pranks   Pranks
}

func New(items *items.Service, policies dto.Access, sessions *sessions.Service, channels *channels.Service, pranks *prank.Service) *Server {
	server := &Server{items: items, policies: policies, sessions: sessions}
	if pranks != nil {
		server.pranks = pranks
	}
	if channels != nil {
		server.channels = channels
	}

	return server
}

type asked struct {
	profile    api.DeviceProfile
	startTicks int64
	noDirect   bool
}

func (s *Server) GetPlaybackInfo(ctx context.Context, request api.GetPlaybackInfoRequestObject) (api.GetPlaybackInfoResponseObject, error) {
	response, err := s.playbackInfo(ctx, request.ItemId, asked{})
	if err != nil {
		return nil, err
	}

	return api.GetPlaybackInfo200JSONResponse(response), nil
}

func (s *Server) GetPostedPlaybackInfo(ctx context.Context, request api.GetPostedPlaybackInfoRequestObject) (api.GetPostedPlaybackInfoResponseObject, error) {
	question := asked{
		startTicks: apiutil.Deref(request.Params.StartTimeTicks),
		noDirect:   !apiutil.OrElse(request.Params.EnableDirectPlay, true),
	}
	if body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody); body != nil {
		if body.DeviceProfile != nil {
			question.profile = *body.DeviceProfile
		}
		if body.StartTimeTicks != nil {
			question.startTicks = *body.StartTimeTicks
		}
		if body.EnableDirectPlay != nil && !*body.EnableDirectPlay {
			question.noDirect = true
		}
	}

	response, err := s.playbackInfo(ctx, request.ItemId, question)
	if err != nil {
		return nil, err
	}

	return api.GetPostedPlaybackInfo200JSONResponse(response), nil
}

func (s *Server) playbackInfo(ctx context.Context, itemID uuid.UUID, question asked) (api.PlaybackInfoResponse, error) {
	if len(question.profile) == 0 {
		question.profile = sessions.Profile(auth.SessionFrom(ctx))
	}

	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	session, err := newPlaySessionId()
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	if s.channels != nil {
		channel, err := s.channels.Channel(ctx, viewer, itemID)
		if err == nil {
			return s.tuned(ctx, channel, session)
		}
		if !errors.Is(err, channels.ErrNotFound) {
			return api.PlaybackInfoResponse{}, err
		}
	}

	item, err := s.items.ItemByID(ctx, viewer, itemID)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	response := api.PlaybackInfoResponse{
		MediaSources:  &[]api.MediaSourceInfo{},
		PlaySessionId: apiutil.Ptr(session),
	}
	played, standsIn, err := s.standIn(ctx, item)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	plan, err := s.items.SourceFor(ctx, played, capabilities(question.profile))
	if errors.Is(err, items.ErrNoSource) || errors.Is(err, items.ErrNoPlayable) {
		log.Printf("nothing to play for %s: %v", item.Name, err)
		response.ErrorCode = apiutil.Ptr(api.NoCompatibleStream)

		return response, nil
	}
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	playback := plan.Playback()
	if standsIn {
		playback.StandsInFor = &item.ID
	}
	s.record(ctx, plan, playback)

	token := auth.AuthorizationFrom(ctx).Token
	source := dto.MediaSourceDto(plan.Source)
	switch {
	case items.IsAudio(item):
		served(&source, plan, streamURL("Audio", itemID, plan, token, session, question.startTicks))
	case plan.Change == items.ChangeNone:
		served(&source, plan, streamURL("Videos", itemID, plan, token, session, question.startTicks))
		source.SupportsDirectPlay = apiutil.Ptr(!question.noDirect)
	default:
		segmented, err := s.segmented(ctx, plan)
		if err != nil {
			return api.PlaybackInfoResponse{}, err
		}
		if !segmented {
			served(&source, plan, streamURL("Videos", itemID, plan, token, session, question.startTicks))
			break
		}
		served(&source, plan, playlistURL(itemID, plan, token, session, question.startTicks))
		source.TranscodingSubProtocol = apiutil.Ptr(api.MediaStreamProtocolHls)
		source.TranscodingContainer = apiutil.Ptr(transcode.VideoContainer)
	}

	response.MediaSources = &[]api.MediaSourceInfo{source}

	return response, nil
}

func (s *Server) standIn(ctx context.Context, item *items.Item) (*items.Item, bool, error) {
	session := auth.SessionFrom(ctx)
	if s.pranks == nil || session == nil || session.UserID == nil {
		return item, false, nil
	}

	return s.pranks.Swap(ctx, *session.UserID, item)
}

func (s *Server) record(ctx context.Context, plan items.Plan, playback items.Playback) {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return
	}
	if err := s.sessions.Planned(ctx, session.ID, plan.Source.ID, playback); err != nil {
		log.Printf("failed to record how %s is served: %v", plan.Source.ID, err)
	}
}

func served(dto *api.MediaSourceInfo, plan items.Plan, url string) {
	dto.SupportsDirectPlay = apiutil.Ptr(false)
	dto.SupportsDirectStream = apiutil.Ptr(false)
	dto.SupportsTranscoding = apiutil.Ptr(true)
	dto.TranscodingUrl = apiutil.Ptr(url)
	dto.TranscodingContainer = apiutil.Ptr(plan.Container)
}

func (s *Server) segmented(ctx context.Context, plan items.Plan) (bool, error) {
	if plan.Change.EncodesVideo() {
		return true, nil
	}
	if !transcode.CopiesVideo(items.VideoCodec(plan.Source)) {
		return false, nil
	}

	keyframes, err := s.items.Keyframes(ctx, plan.Source.ID)

	return len(keyframes) > 0, err
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
