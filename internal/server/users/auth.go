package users

import (
	"context"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func (s *Server) AuthenticateUserByName(ctx context.Context, request api.AuthenticateUserByNameRequestObject) (api.AuthenticateUserByNameResponseObject, error) {
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Username == nil {
		return nil, middleware.ErrUnauthorized
	}

	user, err := s.users.UserByUsername(ctx, *req.Username)
	if err != nil {
		return nil, middleware.ErrUnauthorized
	}

	matches, err := auth.Verify(deref(req.Pw), user.PasswordHash)
	if err != nil || !matches {
		return nil, middleware.ErrUnauthorized
	}

	token, err := auth.NewToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	authorization := middleware.AuthorizationFrom(ctx)
	session := &users.Session{
		UserID:           user.ID,
		AccessToken:      token,
		DeviceID:         authorization.DeviceID,
		DeviceName:       authorization.Device,
		Client:           authorization.Client,
		AppVersion:       authorization.Version,
		LastActivityDate: now,
	}
	if err := s.users.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	if err := s.users.TouchLogin(ctx, user.ID); err != nil {
		return nil, err
	}
	user.LastLoginDate = &now
	user.LastActivityDate = &now

	dto, err := userDto(user)
	if err != nil {
		return nil, err
	}

	return api.AuthenticateUserByName200JSONResponse{
		AccessToken: ptr(token),
		ServerId:    ptr(serverId),
		User:        &dto,
		SessionInfo: SessionDto(session, user),
	}, nil
}

func SessionDto(session *users.Session, user *users.User) *api.SessionInfoDto {
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
