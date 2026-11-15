package mediainfo

import (
	"context"
	"errors"
	"io"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

type Server struct {
	policies dto.Access
	items    *items.Service
	sessions *sessions.Service
}

func New(items *items.Service, policies dto.Access, sessions *sessions.Service) *Server {
	return &Server{items: items, policies: policies, sessions: sessions}
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
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	item, err := s.items.ItemByID(ctx, viewer, itemID)
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
	native := buildsItsOwnURL(profile)

	plan, err := s.items.SourceFor(ctx, item, capabilities(profile))
	if errors.Is(err, items.ErrNoSource) || errors.Is(err, items.ErrNoPlayable) {
		log.Printf("nothing to play for %s: %v", item.Name, err)
		response.ErrorCode = apiutil.Ptr(api.NoCompatibleStream)

		return response, nil
	}
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}

	s.record(ctx, plan)

	root := "Videos"
	if items.IsAudio(item) {
		root = "Audio"
	}

	source := dto.MediaSourceDto(plan.Source)
	if native {
		response.PlaySessionId = apiutil.Ptr(offered(&source, plan, session))
	} else {
		served(&source, plan, streamURL(root, itemID, plan, auth.AuthorizationFrom(ctx).Token, session, startTicks))
	}

	response.MediaSources = &[]api.MediaSourceInfo{source}

	return response, nil
}

func (s *Server) record(ctx context.Context, plan items.Plan) {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return
	}
	if err := s.sessions.Planned(ctx, session.ID, plan.Source.ID, plan.Playback()); err != nil {
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

func offered(dto *api.MediaSourceInfo, plan items.Plan, session string) string {
	dto.SupportsTranscoding = apiutil.Ptr(false)
	if plan.Change == items.ChangeNone {
		dto.SupportsDirectStream = apiutil.Ptr(false)

		return session
	}

	dto.SupportsDirectPlay = apiutil.Ptr(false)
	dto.Container = apiutil.Ptr(plan.Container)
	instruction := conversion(plan)
	instruction.Set(sessionKey, session)

	return instruction.Encode()
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
