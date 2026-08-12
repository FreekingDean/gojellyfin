package playstate

import (
	"context"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/google/uuid"
)

// Below this fraction of the runtime a stop is treated as an abandoned resume
// point; above it the item counts as watched.
const playedThreshold = 0.9

func (s *Server) userItemDatum(ctx context.Context, itemID uuid.UUID) (*items.Datum, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return nil, auth.ErrUnauthorized
	}

	return s.items.UserItemDatum(ctx, userID, itemID)
}

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) ReportPlaybackStart(ctx context.Context, request api.ReportPlaybackStartRequestObject) (api.ReportPlaybackStartResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req != nil && req.ItemId != nil {
		if err := s.recordProgress(ctx, *req.ItemId, apiutil.Deref(req.PositionTicks)); err != nil {
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

	return api.ReportPlaybackStopped204Response{}, nil
}

/*
func (s *Server) OnPlaybackStart(ctx context.Context, request api.OnPlaybackStartRequestObject) (api.OnPlaybackStartResponseObject, error) {
	if err := s.recordProgress(ctx, request.ItemId, 0); err != nil {
		return nil, err
	}

	return api.OnPlaybackStart204Response{}, nil
}

func (s *Server) OnPlaybackProgress(ctx context.Context, request api.OnPlaybackProgressRequestObject) (api.OnPlaybackProgressResponseObject, error) {
	if err := s.recordProgress(ctx, request.ItemId, apiutil.Deref(request.Params.PositionTicks)); err != nil {
		return nil, err
	}

	return api.OnPlaybackProgress204Response{}, nil
}

func (s *Server) OnPlaybackStopped(ctx context.Context, request api.OnPlaybackStoppedRequestObject) (api.OnPlaybackStoppedResponseObject, error) {
	if err := s.recordStop(ctx, request.ItemId, apiutil.Deref(request.Params.PositionTicks)); err != nil {
		return nil, err
	}

	return api.OnPlaybackStopped204Response{}, nil
}
*/

func (s *Server) PingPlaybackSession(ctx context.Context, request api.PingPlaybackSessionRequestObject) (api.PingPlaybackSessionResponseObject, error) {
	return api.PingPlaybackSession204Response{}, nil
}

func (s *Server) recordProgress(ctx context.Context, itemID uuid.UUID, position int64) error {
	datum, err := s.userItemDatum(ctx, itemID)
	if err != nil {
		return err
	}

	datum.PlaybackPositionTicks = position
	datum.LastPlayedAt = apiutil.Ptr(time.Now())

	return s.items.SaveUserItemDatum(ctx, datum)
}

func (s *Server) recordStop(ctx context.Context, itemID uuid.UUID, position int64) error {
	datum, err := s.userItemDatum(ctx, itemID)
	if err != nil {
		return err
	}

	item, err := s.items.ItemByID(ctx, itemID)
	if err != nil {
		return err
	}

	datum.LastPlayedAt = apiutil.Ptr(time.Now())
	datum.PlaybackPositionTicks = position

	if watched(item.RunTimeTicks, position) {
		datum.Played = true
		datum.PlayCount++
		datum.PlaybackPositionTicks = 0
	}

	return s.items.SaveUserItemDatum(ctx, datum)
}

func watched(runTimeTicks *int64, position int64) bool {
	if runTimeTicks == nil || *runTimeTicks == 0 {
		return false
	}

	return float64(position)/float64(*runTimeTicks) >= playedThreshold
}
