package user

import (
	"context"
	"encoding/json"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	users *users.Service
	auth  *auth.Service
}

func New(users *users.Service, auth *auth.Service) *Server {
	return &Server{users: users, auth: auth}
}

func (s *Server) GetUsers(ctx context.Context, request api.GetUsersRequestObject) (api.GetUsersResponseObject, error) {
	converted, err := s.listUserDtos(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetUsers200JSONResponse(converted), nil
}

func (s *Server) GetCurrentUser(ctx context.Context, request api.GetCurrentUserRequestObject) (api.GetCurrentUserResponseObject, error) {
	user, err := s.users.User(ctx, middleware.UserID(ctx))
	if err != nil {
		return api.GetCurrentUser400JSONResponse{}, nil
	}

	dto, err := dtos.UserDto(user)
	if err != nil {
		return nil, err
	}

	return api.GetCurrentUser200JSONResponse(dto), nil
}

func (s *Server) GetUserById(ctx context.Context, request api.GetUserByIdRequestObject) (api.GetUserByIdResponseObject, error) {
	user, err := s.users.User(ctx, request.UserId)
	if err != nil {
		return api.GetUserById404JSONResponse{}, nil
	}

	dto, err := dtos.UserDto(user)
	if err != nil {
		return nil, err
	}

	return api.GetUserById200JSONResponse(dto), nil
}

func (s *Server) GetPublicUsers(ctx context.Context, request api.GetPublicUsersRequestObject) (api.GetPublicUsersResponseObject, error) {
	converted, err := s.listUserDtos(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetPublicUsers200JSONResponse(converted), nil
}

func (s *Server) CreateUserByName(ctx context.Context, request api.CreateUserByNameRequestObject) (api.CreateUserByNameResponseObject, error) {
	req := dtos.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.CreateUserByName403Response{}, nil
	}

	hash, err := auth.Hash(dtos.Deref(req.Password))
	if err != nil {
		return nil, err
	}

	user := &users.User{
		Name:         req.Name,
		Username:     req.Name,
		PasswordHash: hash,
	}
	if err := s.users.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	dto, err := dtos.UserDto(user)
	if err != nil {
		return nil, err
	}

	return api.CreateUserByName200JSONResponse(dto), nil
}

func (s *Server) UpdateUser(ctx context.Context, request api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	req := dtos.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || request.Params.UserId == nil {
		return api.UpdateUser400JSONResponse{}, nil
	}

	user, err := s.users.User(ctx, *request.Params.UserId)
	if err != nil {
		return api.UpdateUser400JSONResponse{}, nil
	}

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Configuration != nil {
		if user.Configuration, err = json.Marshal(req.Configuration); err != nil {
			return nil, err
		}
	}
	if req.Policy != nil {
		if user.Policy, err = json.Marshal(req.Policy); err != nil {
			return nil, err
		}
		user.IsAdministrator = dtos.Deref(req.Policy.IsAdministrator)
	}

	if err := s.users.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUser204Response{}, nil
}

func (s *Server) UpdateUserConfiguration(ctx context.Context, request api.UpdateUserConfigurationRequestObject) (api.UpdateUserConfigurationResponseObject, error) {
	req := dtos.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateUserConfiguration403JSONResponse{}, nil
	}

	user, err := s.userFor(ctx, request.Params.UserId)
	if err != nil {
		return api.UpdateUserConfiguration403JSONResponse{}, nil
	}

	if user.Configuration, err = json.Marshal(req); err != nil {
		return nil, err
	}
	if err := s.users.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUserConfiguration204Response{}, nil
}

func (s *Server) UpdateUserPolicy(ctx context.Context, request api.UpdateUserPolicyRequestObject) (api.UpdateUserPolicyResponseObject, error) {
	req := dtos.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateUserPolicy400JSONResponse{}, nil
	}

	user, err := s.users.User(ctx, request.UserId)
	if err != nil {
		return api.UpdateUserPolicy400JSONResponse{}, nil
	}

	if user.Policy, err = json.Marshal(req); err != nil {
		return nil, err
	}
	user.IsAdministrator = dtos.Deref(req.IsAdministrator)

	if err := s.users.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUserPolicy204Response{}, nil
}

func (s *Server) UpdateUserPassword(ctx context.Context, request api.UpdateUserPasswordRequestObject) (api.UpdateUserPasswordResponseObject, error) {
	req := dtos.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateUserPassword403JSONResponse{}, nil
	}

	user, err := s.userFor(ctx, request.Params.UserId)
	if err != nil {
		return api.UpdateUserPassword404JSONResponse{}, nil
	}

	if !dtos.Deref(req.ResetPassword) {
		matches, err := auth.Verify(dtos.Deref(req.CurrentPw), user.PasswordHash)
		if err != nil || !matches {
			return api.UpdateUserPassword403JSONResponse{}, nil
		}
	}

	if user.PasswordHash, err = auth.Hash(dtos.Deref(req.NewPw)); err != nil {
		return nil, err
	}
	if err := s.users.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUserPassword204Response{}, nil
}

func (s *Server) DeleteUser(ctx context.Context, request api.DeleteUserRequestObject) (api.DeleteUserResponseObject, error) {
	if _, err := s.users.User(ctx, request.UserId); err != nil {
		return api.DeleteUser404JSONResponse{}, nil
	}

	if err := s.users.DeleteUser(ctx, request.UserId); err != nil {
		return nil, err
	}

	return api.DeleteUser204Response{}, nil
}

func (s *Server) listUserDtos(ctx context.Context) ([]api.UserDto, error) {
	users, err := s.users.Users(ctx)
	if err != nil {
		return nil, err
	}

	converted := make([]api.UserDto, 0, len(users))
	for _, user := range users {
		dto, err := dtos.UserDto(&user)
		if err != nil {
			return nil, err
		}
		converted = append(converted, dto)
	}

	return converted, nil
}

func (s *Server) userFor(ctx context.Context, id *openapi_types.UUID) (*users.User, error) {
	if id == nil {
		return s.users.User(ctx, middleware.UserID(ctx))
	}

	return s.users.User(ctx, *id)
}

func (s *Server) AuthenticateUserByName(ctx context.Context, request api.AuthenticateUserByNameRequestObject) (api.AuthenticateUserByNameResponseObject, error) {
	req := dtos.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Username == nil {
		return nil, middleware.ErrUnauthorized
	}

	user, err := s.users.UserByUsername(ctx, *req.Username)
	if err != nil {
		return nil, middleware.ErrUnauthorized
	}

	matches, err := auth.Verify(dtos.Deref(req.Pw), user.PasswordHash)
	if err != nil || !matches {
		return nil, middleware.ErrUnauthorized
	}

	token, err := auth.NewToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	authorization := middleware.AuthorizationFrom(ctx)
	session := &auth.Session{
		UserID:           user.ID,
		AccessToken:      token,
		DeviceID:         authorization.DeviceID,
		DeviceName:       authorization.Device,
		Client:           authorization.Client,
		AppVersion:       authorization.Version,
		LastActivityDate: now,
	}
	if err := s.auth.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	if err := s.users.TouchLogin(ctx, user.ID); err != nil {
		return nil, err
	}
	user.LastLoginDate = &now
	user.LastActivityDate = &now

	dto, err := dtos.UserDto(user)
	if err != nil {
		return nil, err
	}

	return api.AuthenticateUserByName200JSONResponse{
		AccessToken: dtos.Ptr(token),
		ServerId:    dtos.Ptr(config.ServerID),
		User:        &dto,
		SessionInfo: dtos.SessionDto(session, user),
	}, nil
}
