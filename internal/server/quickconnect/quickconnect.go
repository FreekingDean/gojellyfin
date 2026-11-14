package quickconnect

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/link"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
type QuickConnectService interface {
	Initiate(ctx context.Context, device sessions.Device) (*quickconnect.Request, error)
	Pending(ctx context.Context, secret string) (*quickconnect.Request, error)
	Authorize(ctx context.Context, code string, userID uuid.UUID) error
}

type UsersService interface {
	IsAdministrator(ctx context.Context, id uuid.UUID) (bool, error)
	User(ctx context.Context, id uuid.UUID) (*users.User, error)
}

type Remembering interface {
	Remember(w http.ResponseWriter, request *quickconnect.Request)
}

type Server struct {
	quickconnect QuickConnectService
	users        UsersService
	codes        Remembering
}

func New(quickconnect *quickconnect.Service, users *users.Service, codes *link.Handler) *Server {
	return &Server{quickconnect: quickconnect, users: users, codes: codes}
}

type initiated struct {
	api.InitiateQuickConnect200JSONResponse
	remember func(http.ResponseWriter)
}

func (response initiated) VisitInitiateQuickConnectResponse(w http.ResponseWriter) error {
	response.remember(w)

	return response.InitiateQuickConnect200JSONResponse.VisitInitiateQuickConnectResponse(w)
}

func (s *Server) GetQuickConnectEnabled(ctx context.Context, request api.GetQuickConnectEnabledRequestObject) (api.GetQuickConnectEnabledResponseObject, error) {
	return api.GetQuickConnectEnabled200JSONResponse(true), nil
}

func (s *Server) InitiateQuickConnect(ctx context.Context, request api.InitiateQuickConnectRequestObject) (api.InitiateQuickConnectResponseObject, error) {
	pending, err := s.quickconnect.Initiate(ctx, auth.AuthorizationFrom(ctx).ClientDevice())
	if err != nil {
		return nil, err
	}

	return initiated{
		InitiateQuickConnect200JSONResponse: api.InitiateQuickConnect200JSONResponse(resultDto(pending)),
		remember:                            func(w http.ResponseWriter) { s.codes.Remember(w, pending) },
	}, nil
}

func (s *Server) GetQuickConnectState(ctx context.Context, request api.GetQuickConnectStateRequestObject) (api.GetQuickConnectStateResponseObject, error) {
	pending, err := s.quickconnect.Pending(ctx, request.Params.Secret)
	if errors.Is(err, quickconnect.ErrNotFound) {
		return api.GetQuickConnectState404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetQuickConnectState200JSONResponse(resultDto(pending)), nil
}

func (s *Server) AuthorizeQuickConnect(ctx context.Context, request api.AuthorizeQuickConnectRequestObject) (api.AuthorizeQuickConnectResponseObject, error) {
	userID := auth.UserID(ctx)

	if target := request.Params.UserId; target != nil && *target != userID {
		administrator, err := s.users.IsAdministrator(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !administrator {
			return api.AuthorizeQuickConnect403JSONResponse{}, nil
		}
		if _, err := s.users.User(ctx, *target); err != nil {
			return api.AuthorizeQuickConnect403JSONResponse{}, nil
		}
		userID = *target
	}

	err := s.quickconnect.Authorize(ctx, request.Params.Code, userID)
	if errors.Is(err, quickconnect.ErrNotFound) {
		return api.AuthorizeQuickConnect200JSONResponse(false), nil
	}
	if err != nil {
		return nil, err
	}

	return api.AuthorizeQuickConnect200JSONResponse(true), nil
}
