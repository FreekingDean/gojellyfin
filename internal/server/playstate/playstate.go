package playstate

import (
	"context"
	"errors"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/google/uuid"
)

type Channels interface {
	Channel(ctx context.Context, viewer items.Viewer, id uuid.UUID) (*channels.Channel, error)
	Schedule(ctx context.Context, channel *channels.Channel) (channels.Schedule, error)
	Now() time.Time
}

type Server struct {
	channels Channels
	items    *items.Service
	sessions *sessions.Service
}

func New(items *items.Service, sessions *sessions.Service, channels *channels.Service) *Server {
	server := &Server{items: items, sessions: sessions}
	if channels != nil {
		server.channels = channels
	}

	return server
}

func (s *Server) ReportPlaybackStart(ctx context.Context, request api.ReportPlaybackStartRequestObject) (api.ReportPlaybackStartResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req != nil && req.ItemId != nil {
		if err := s.report(ctx, *req.ItemId, req.PositionTicks, req.PlayMethod, req.IsPaused); err != nil {
			return nil, err
		}
	}

	return api.ReportPlaybackStart204Response{}, nil
}

func (s *Server) ReportPlaybackProgress(ctx context.Context, request api.ReportPlaybackProgressRequestObject) (api.ReportPlaybackProgressResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req != nil && req.ItemId != nil {
		if err := s.report(ctx, *req.ItemId, req.PositionTicks, req.PlayMethod, req.IsPaused); err != nil {
			return nil, err
		}
	}

	return api.ReportPlaybackProgress204Response{}, nil
}

func (s *Server) ReportPlaybackStopped(ctx context.Context, request api.ReportPlaybackStoppedRequestObject) (api.ReportPlaybackStoppedResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req != nil && req.ItemId != nil {
		_, live, err := s.onAir(ctx, *req.ItemId)
		if err != nil {
			return nil, err
		}
		if !live {
			if err := s.recordStop(ctx, *req.ItemId, apiutil.Deref(req.PositionTicks)); err != nil {
				return nil, err
			}
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

func (s *Server) report(ctx context.Context, itemID uuid.UUID, position *int64, method *api.PlayMethod, paused *bool) error {
	airing, live, err := s.onAir(ctx, itemID)
	if err != nil {
		return err
	}
	if live {
		if airing == nil {
			return nil
		}
		ticks := int64(s.channels.Now().Sub(airing.Start) / (100 * time.Nanosecond))

		return s.playing(ctx, airing.ItemID, &ticks, method, paused)
	}

	if err := s.recordProgress(ctx, itemID, apiutil.Deref(position)); err != nil {
		return err
	}

	return s.playing(ctx, itemID, position, method, paused)
}

func (s *Server) onAir(ctx context.Context, itemID uuid.UUID) (*channels.Airing, bool, error) {
	if s.channels == nil {
		return nil, false, nil
	}

	channel, err := s.channels.Channel(ctx, items.Everyone, itemID)
	if errors.Is(err, channels.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	schedule, err := s.channels.Schedule(ctx, channel)
	if err != nil {
		return nil, false, err
	}
	airing, found := schedule.At(s.channels.Now())
	if !found {
		return nil, true, nil
	}

	return &airing, true, nil
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
