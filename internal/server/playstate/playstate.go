package playstate

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/google/uuid"
)

type Server struct {
	items    *items.Service
	sessions *sessions.Service
}

func New(items *items.Service, sessions *sessions.Service) *Server {
	return &Server{items: items, sessions: sessions}
}

func (s *Server) ReportPlaybackStart(ctx context.Context, request api.ReportPlaybackStartRequestObject) (api.ReportPlaybackStartResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req != nil && req.ItemId != nil {
		if err := s.recordProgress(ctx, *req.ItemId, apiutil.Deref(req.PositionTicks)); err != nil {
			return nil, err
		}
		if err := s.playing(ctx, *req.ItemId, req.PositionTicks, req.PlayMethod, req.IsPaused); err != nil {
			return nil, err
		}
	}

	return api.ReportPlaybackStart204Response{}, nil
}

func (s *Server) ReportPlaybackProgress(ctx context.Context, request api.ReportPlaybackProgressRequestObject) (api.ReportPlaybackProgressResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req != nil && req.ItemId != nil {
		if err := s.recordProgress(ctx, *req.ItemId, apiutil.Deref(req.PositionTicks)); err != nil {
			return nil, err
		}
		if err := s.playing(ctx, *req.ItemId, req.PositionTicks, req.PlayMethod, req.IsPaused); err != nil {
			return nil, err
		}
	}

	return api.ReportPlaybackProgress204Response{}, nil
}

func (s *Server) ReportPlaybackStopped(ctx context.Context, request api.ReportPlaybackStoppedRequestObject) (api.ReportPlaybackStoppedResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req != nil && req.ItemId != nil {
		if err := s.recordStop(ctx, *req.ItemId, apiutil.Deref(req.PositionTicks)); err != nil {
			return nil, err
		}
	}
	if session := auth.SessionFrom(ctx); session != nil {
		if err := s.sessions.Stopped(ctx, session.ID, auth.AuthorizationFrom(ctx).RemoteAddr); err != nil {
			return nil, err
		}
	}

	return api.ReportPlaybackStopped204Response{}, nil
}

func (s *Server) PingPlaybackSession(ctx context.Context, request api.PingPlaybackSessionRequestObject) (api.PingPlaybackSessionResponseObject, error) {
	return api.PingPlaybackSession204Response{}, nil
}

func (s *Server) playing(ctx context.Context, itemID uuid.UUID, position *int64, method *api.PlayMethod, paused *bool) error {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return nil
	}

	return s.sessions.Playing(ctx, session.ID, auth.AuthorizationFrom(ctx).RemoteAddr, sessions.Playback{
		ItemID:        itemID,
		PositionTicks: apiutil.Deref(position),
		PlayMethod:    string(apiutil.Deref(method)),
		IsPaused:      apiutil.Deref(paused),
	})
}

func (s *Server) recordProgress(ctx context.Context, itemID uuid.UUID, position int64) error {
	userID, err := dto.SignedIn(ctx)
	if err != nil {
		return err
	}

	return s.items.RecordProgress(ctx, userID, itemID, position)
}

func (s *Server) recordStop(ctx context.Context, itemID uuid.UUID, position int64) error {
	userID, err := dto.SignedIn(ctx)
	if err != nil {
		return err
	}

	return s.items.RecordStop(ctx, userID, itemID, position)
}
