package session

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	auth  *auth.Service
	users *users.Service
}

func New(auth *auth.Service, users *users.Service) *Server {
	return &Server{auth: auth, users: users}
}

func (s *Server) GetSessions(ctx context.Context, request api.GetSessionsRequestObject) (api.GetSessionsResponseObject, error) {
	sessions, err := s.auth.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	converted := make([]api.SessionInfoDto, 0, len(sessions))
	for _, session := range sessions {
		user, err := s.users.User(ctx, session.UserID)
		if err != nil {
			continue
		}
		converted = append(converted, *SessionDto(&session, user))
	}

	return api.GetSessions200JSONResponse(converted), nil
}

func (s *Server) ReportSessionEnded(ctx context.Context, request api.ReportSessionEndedRequestObject) (api.ReportSessionEndedResponseObject, error) {
	authorization := auth.AuthorizationFrom(ctx)
	if err := s.auth.DeleteSessionByToken(ctx, authorization.Token); err != nil {
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
