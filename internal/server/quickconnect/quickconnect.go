package quickconnect

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

const retryAfterSeconds = 10

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
	pending, err := s.quickconnect.Initiate(ctx, auth.AuthorizationFrom(ctx).DeviceInfo())
	if errors.Is(err, quickconnect.ErrTooManyPending) || errors.Is(err, quickconnect.ErrNoCode) {
		return busy(), nil
	}
	if err != nil {
		return nil, err
	}

	return api.InitiateQuickConnect200JSONResponse(resultDto(pending)), nil
}

func (s *Server) GetQuickConnectState(ctx context.Context, request api.GetQuickConnectStateRequestObject) (api.GetQuickConnectStateResponseObject, error) {
	pending, err := s.quickconnect.Pending(ctx, request.Params.Secret)
	if errors.Is(err, quickconnect.ErrUnknownSecret) {
		return api.GetQuickConnectState404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetQuickConnectState200JSONResponse(resultDto(pending)), nil
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

	err := s.quickconnect.Authorize(ctx, request.Params.Code, userID)
	if errors.Is(err, quickconnect.ErrUnknownCode) || errors.Is(err, quickconnect.ErrAlreadyAuthorized) {
		return api.AuthorizeQuickConnect200JSONResponse(false), nil
	}
	if err != nil {
		return nil, err
	}

	return api.AuthorizeQuickConnect200JSONResponse(true), nil
}

func (s *Server) mayAuthorizeFor(ctx context.Context, userID, target uuid.UUID) error {
	administrator, err := s.users.IsAdministrator(ctx, userID)
	if err != nil {
		return err
	}
	if !administrator {
		return auth.ErrUnauthorized
	}

	_, err = s.users.User(ctx, target)

	return err
}

func busy() api.InitiateQuickConnect503TexthtmlResponse {
	message := "too many pending quick connect requests"

	return api.InitiateQuickConnect503TexthtmlResponse{
		Body:          strings.NewReader(message),
		ContentLength: int64(len(message)),
		Headers: api.InitiateQuickConnect503ResponseHeaders{
			Message:    apiutil.Ptr(message),
			RetryAfter: apiutil.Ptr(int32(retryAfterSeconds)),
		},
	}
}
