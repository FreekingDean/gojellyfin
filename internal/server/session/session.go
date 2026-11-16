package session

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Administrators interface {
	IsAdministrator(ctx context.Context, id uuid.UUID) (bool, error)
}

type Server struct {
	sessions       *sessions.Service
	administrators Administrators
}

func New(sessions *sessions.Service, users *users.Service) *Server {
	return &Server{sessions: sessions, administrators: users}
}

func (s *Server) GetSessions(ctx context.Context, request api.GetSessionsRequestObject) (api.GetSessionsResponseObject, error) {
	if request.Params.ControllableByUserId != nil {
		return api.GetSessions200JSONResponse{}, nil
	}

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
	err := s.report(ctx, api.ClientCapabilitiesDto{
		PlayableMediaTypes:           request.Params.PlayableMediaTypes,
		SupportedCommands:            request.Params.SupportedCommands,
		SupportsMediaControl:         request.Params.SupportsMediaControl,
		SupportsPersistentIdentifier: request.Params.SupportsPersistentIdentifier,
	})
	if err != nil {
		return nil, err
	}

	return api.PostCapabilities204Response{}, nil
}

func (s *Server) PostFullCapabilities(ctx context.Context, request api.PostFullCapabilitiesRequestObject) (api.PostFullCapabilitiesResponseObject, error) {
	if body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody); body != nil {
		if err := s.report(ctx, *body); err != nil {
			return nil, err
		}
	}

	return api.PostFullCapabilities204Response{}, nil
}

func (s *Server) report(ctx context.Context, reported api.ClientCapabilitiesDto) error {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return nil
	}

	return s.sessions.ReportCapabilities(ctx, session, capabilities(reported))
}

func (s *Server) AddUserToSession(ctx context.Context, request api.AddUserToSessionRequestObject) (api.AddUserToSessionResponseObject, error) {
	id, allowed, err := s.owned(ctx, request.SessionId)
	if err != nil || !allowed {
		return api.AddUserToSession403Response{}, err
	}

	err = s.sessions.AddUser(ctx, id, request.UserId)
	if errors.Is(err, sessions.ErrNoSession) || errors.Is(err, sessions.ErrNoUser) || errors.Is(err, sessions.ErrPrimaryUser) {
		return api.AddUserToSession403Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.AddUserToSession204Response{}, nil
}

func (s *Server) RemoveUserFromSession(ctx context.Context, request api.RemoveUserFromSessionRequestObject) (api.RemoveUserFromSessionResponseObject, error) {
	id, allowed, err := s.owned(ctx, request.SessionId)
	if err != nil || !allowed {
		return api.RemoveUserFromSession403Response{}, err
	}

	err = s.sessions.RemoveUser(ctx, id, request.UserId)
	if errors.Is(err, sessions.ErrNoSession) {
		return api.RemoveUserFromSession403Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.RemoveUserFromSession204Response{}, nil
}

func (s *Server) ReportViewing(ctx context.Context, request api.ReportViewingRequestObject) (api.ReportViewingResponseObject, error) {
	session := auth.SessionFrom(ctx)
	itemID, err := uuid.Parse(request.Params.ItemId)
	if session == nil || err != nil {
		return api.ReportViewing403Response{}, nil
	}

	err = s.sessions.Viewing(ctx, session.ID, itemID)
	if errors.Is(err, sessions.ErrNoSuchItem) {
		return api.ReportViewing403Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.ReportViewing204Response{}, nil
}

func (s *Server) owned(ctx context.Context, sessionID string) (uuid.UUID, bool, error) {
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.Nil, false, nil
	}

	caller := auth.UserID(ctx)
	if own := auth.SessionFrom(ctx); own != nil && own.ID == id {
		return id, true, nil
	}

	session, err := s.sessions.ByID(ctx, id)
	if errors.Is(err, sessions.ErrNoSession) {
		return id, false, nil
	}
	if err != nil {
		return id, false, err
	}
	if session.User != nil && session.User.ID == caller {
		return id, true, nil
	}

	administrator, err := s.administrators.IsAdministrator(ctx, caller)

	return id, administrator, err
}
