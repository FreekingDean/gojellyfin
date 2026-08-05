package users

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GetSessions(ctx context.Context, request api.GetSessionsRequestObject) (api.GetSessionsResponseObject, error) {
	sessions, err := s.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	dtos := make([]api.SessionInfoDto, 0, len(sessions))
	for _, session := range sessions {
		user, err := s.user(ctx, session.UserID)
		if err != nil {
			continue
		}
		dtos = append(dtos, *SessionDto(&session, user))
	}

	return api.GetSessions200JSONResponse(dtos), nil
}

func (s *Server) ReportSessionEnded(ctx context.Context, request api.ReportSessionEndedRequestObject) (api.ReportSessionEndedResponseObject, error) {
	authorization := middleware.AuthorizationFrom(ctx)
	if err := s.DeleteSessionByToken(ctx, authorization.Token); err != nil {
		return nil, err
	}

	return api.ReportSessionEnded204Response{}, nil
}

func (s *Server) PostCapabilities(ctx context.Context, request api.PostCapabilitiesRequestObject) (api.PostCapabilitiesResponseObject, error) {
	return api.PostCapabilities204Response{}, nil
}

func (s *Server) PostFullCapabilities(ctx context.Context, request api.PostFullCapabilitiesRequestObject) (api.PostFullCapabilitiesResponseObject, error) {
	return api.PostFullCapabilities204Response{}, nil
}
