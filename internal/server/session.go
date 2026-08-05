package server

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

func (s *Server) GetSessions(ctx context.Context, request api.GetSessionsRequestObject) (api.GetSessionsResponseObject, error) {
	sessions, err := s.store.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	dtos := make([]api.SessionInfoDto, 0, len(sessions))
	for _, session := range sessions {
		user, err := s.store.GetUser(ctx, session.UserID)
		if err != nil {
			continue
		}
		dtos = append(dtos, *sessionDto(&session, user))
	}

	return api.GetSessions200JSONResponse(dtos), nil
}

func (s *Server) ReportSessionEnded(ctx context.Context, request api.ReportSessionEndedRequestObject) (api.ReportSessionEndedResponseObject, error) {
	authorization := middleware.AuthorizationFrom(ctx)
	if err := s.store.DeleteSessionByToken(ctx, authorization.Token); err != nil {
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

func sessionDto(session *store.Session, user *store.User) *api.SessionInfoDto {
	dto := &api.SessionInfoDto{
		Id:                    ptr(session.ID.String()),
		ServerId:              ptr(serverId),
		Client:                ptr(session.Client),
		DeviceId:              ptr(session.DeviceID),
		DeviceName:            ptr(session.DeviceName),
		ApplicationVersion:    ptr(session.AppVersion),
		LastActivityDate:      ptr(session.LastActivityDate),
		IsActive:              ptr(true),
		SupportsRemoteControl: ptr(false),
		PlayableMediaTypes:    &[]api.MediaType{},
		SupportedCommands:     &[]api.GeneralCommandType{},
		AdditionalUsers:       &[]api.SessionUserInfo{},
	}

	if user != nil {
		dto.UserId = &user.ID
		dto.UserName = ptr(user.Name)
	}

	return dto
}
