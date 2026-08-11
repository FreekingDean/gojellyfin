package quickconnect

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	quickconnect *quickconnect.Service
	users        *users.Service
}

func New(quickconnect *quickconnect.Service, users *users.Service) *Server {
	return &Server{quickconnect: quickconnect, users: users}
}

func (s *Server) GetQuickConnectEnabled(ctx context.Context, request api.GetQuickConnectEnabledRequestObject) (api.GetQuickConnectEnabledResponseObject, error) {
	return api.GetQuickConnectEnabled200JSONResponse(quickconnect.Enabled), nil
}

func (s *Server) InitiateQuickConnect(ctx context.Context, request api.InitiateQuickConnectRequestObject) (api.InitiateQuickConnectResponseObject, error) {
	pending, err := s.quickconnect.Initiate(auth.AuthorizationFrom(ctx).DeviceInfo())
	if err != nil {
		return nil, err
	}

	return api.InitiateQuickConnect200JSONResponse(ResultDto(pending)), nil
}

func (s *Server) GetQuickConnectState(ctx context.Context, request api.GetQuickConnectStateRequestObject) (api.GetQuickConnectStateResponseObject, error) {
	pending, err := s.quickconnect.Pending(request.Params.Secret)
	if err != nil {
		return api.GetQuickConnectState404JSONResponse{}, nil
	}

	return api.GetQuickConnectState200JSONResponse(ResultDto(pending)), nil
}

func (s *Server) AuthorizeQuickConnect(ctx context.Context, request api.AuthorizeQuickConnectRequestObject) (api.AuthorizeQuickConnectResponseObject, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return nil, auth.ErrUnauthorized
	}

	if target := request.Params.UserId; target != nil && *target != userID {
		if err := s.mayAuthorizeFor(ctx, userID, *target); err != nil {
			return api.AuthorizeQuickConnect403JSONResponse{}, nil
		}
		userID = *target
	}

	if err := s.quickconnect.Authorize(request.Params.Code, userID); err != nil {
		return api.AuthorizeQuickConnect200JSONResponse(false), nil
	}

	return api.AuthorizeQuickConnect200JSONResponse(true), nil
}

func (s *Server) mayAuthorizeFor(ctx context.Context, userID, target uuid.UUID) error {
	caller, err := s.users.User(ctx, userID)
	if err != nil {
		return err
	}
	if caller.Edges.Policy == nil || !caller.Edges.Policy.IsAdministrator {
		return auth.ErrUnauthorized
	}

	_, err = s.users.User(ctx, target)

	return err
}
