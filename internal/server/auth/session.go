package auth

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GetSessions(ctx context.Context, request api.GetSessionsRequestObject) (api.GetSessionsResponseObject, error) {
	sessions, err := s.auth.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	dtos := make([]api.SessionInfoDto, 0, len(sessions))
	for _, session := range sessions {
		user, err := s.users.User(ctx, session.UserID)
		if err != nil {
			continue
		}
		dtos = append(dtos, *SessionDto(&session, user))
	}

	return api.GetSessions200JSONResponse(dtos), nil
}

func (s *Server) ReportSessionEnded(ctx context.Context, request api.ReportSessionEndedRequestObject) (api.ReportSessionEndedResponseObject, error) {
	authorization := middleware.AuthorizationFrom(ctx)
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

// Satisfies middleware.Sessions. The translation lives here rather than beside
// the query so storage has no reason to know about transport types.
func (s *Server) SessionByToken(ctx context.Context, token string) (middleware.Session, error) {
	session, err := s.auth.SessionByToken(ctx, token)
	if err != nil {
		return middleware.Session{}, err
	}

	return middleware.Session{ID: session.ID, UserID: session.UserID}, nil
}
