package session

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

type Server struct {
	sessions *sessions.Service
}

func New(sessions *sessions.Service) *Server {
	return &Server{sessions: sessions}
}

func (s *Server) GetSessions(ctx context.Context, request api.GetSessionsRequestObject) (api.GetSessionsResponseObject, error) {
	active, err := s.sessions.List(ctx)
	if err != nil {
		return nil, err
	}

	converted := make([]api.SessionInfoDto, 0, len(active))
	for _, session := range active {
		converted = append(converted, *dto.SessionDto(session))
	}

	return api.GetSessions200JSONResponse(converted), nil
}

func (s *Server) ReportSessionEnded(ctx context.Context, request api.ReportSessionEndedRequestObject) (api.ReportSessionEndedResponseObject, error) {
	authorization := auth.AuthorizationFrom(ctx)
	if err := s.sessions.DeleteByToken(ctx, authorization.Token); err != nil {
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
