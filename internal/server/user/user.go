package user

import (
	"context"
	"encoding/json"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serversession "github.com/FreekingDean/gojellyfin/internal/server/session"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	users    *users.Service
	sessions *sessions.Service
}

func New(users *users.Service, sessions *sessions.Service) *Server {
	return &Server{users: users, sessions: sessions}
}

func (s *Server) GetUsers(ctx context.Context, request api.GetUsersRequestObject) (api.GetUsersResponseObject, error) {
	converted, err := s.listUserDtos(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetUsers200JSONResponse(converted), nil
}

func (s *Server) GetCurrentUser(ctx context.Context, request api.GetCurrentUserRequestObject) (api.GetCurrentUserResponseObject, error) {
	user, err := s.users.User(ctx, auth.UserID(ctx))
	if err != nil {
		return api.GetCurrentUser400JSONResponse{}, nil
	}

	dto, err := UserDto(user)
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

	dto, err := UserDto(user)
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
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.CreateUserByName403Response{}, nil
	}

	hash, err := auth.Hash(apiutil.Deref(req.Password))
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

	dto, err := UserDto(user)
	if err != nil {
		return nil, err
	}

	return api.CreateUserByName200JSONResponse(dto), nil
}

func (s *Server) UpdateUser(ctx context.Context, request api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
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
		user.IsAdministrator = apiutil.Deref(req.Policy.IsAdministrator)
	}

	if err := s.users.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUser204Response{}, nil
}

func (s *Server) UpdateUserConfiguration(ctx context.Context, request api.UpdateUserConfigurationRequestObject) (api.UpdateUserConfigurationResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
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
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
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
	user.IsAdministrator = apiutil.Deref(req.IsAdministrator)

	if err := s.users.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUserPolicy204Response{}, nil
}

func (s *Server) UpdateUserPassword(ctx context.Context, request api.UpdateUserPasswordRequestObject) (api.UpdateUserPasswordResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateUserPassword403JSONResponse{}, nil
	}

	user, err := s.userFor(ctx, request.Params.UserId)
	if err != nil {
		return api.UpdateUserPassword404JSONResponse{}, nil
	}

	if !apiutil.Deref(req.ResetPassword) {
		matches, err := auth.Verify(apiutil.Deref(req.CurrentPw), user.PasswordHash)
		if err != nil || !matches {
			return api.UpdateUserPassword403JSONResponse{}, nil
		}
	}

	if user.PasswordHash, err = auth.Hash(apiutil.Deref(req.NewPw)); err != nil {
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
		dto, err := UserDto(&user)
		if err != nil {
			return nil, err
		}
		converted = append(converted, dto)
	}

	return converted, nil
}

func (s *Server) userFor(ctx context.Context, id *openapi_types.UUID) (*users.User, error) {
	if id == nil {
		return s.users.User(ctx, auth.UserID(ctx))
	}

	return s.users.User(ctx, *id)
}

func (s *Server) AuthenticateUserByName(ctx context.Context, request api.AuthenticateUserByNameRequestObject) (api.AuthenticateUserByNameResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Username == nil {
		return nil, auth.ErrUnauthorized
	}

	user, err := s.users.UserByUsername(ctx, *req.Username)
	if err != nil {
		return nil, auth.ErrUnauthorized
	}

	matches, err := auth.Verify(apiutil.Deref(req.Pw), user.PasswordHash)
	if err != nil || !matches {
		return nil, auth.ErrUnauthorized
	}

	token, err := auth.NewToken()
	if err != nil {
		return nil, err
	}

	session, err := s.sessions.Create(ctx, user.ID, token, auth.AuthorizationFrom(ctx).DeviceInfo())
	if err != nil {
		return nil, err
	}

	if err := s.users.TouchLogin(ctx, user.ID); err != nil {
		return nil, err
	}
	now := time.Now()
	user.LastLoginDate = &now
	user.LastActivityDate = &now

	dto, err := UserDto(user)
	if err != nil {
		return nil, err
	}

	return api.AuthenticateUserByName200JSONResponse{
		AccessToken: apiutil.Ptr(token),
		ServerId:    apiutil.Ptr(config.ServerID),
		User:        &dto,
		SessionInfo: serversession.SessionDto(session),
	}, nil
}
