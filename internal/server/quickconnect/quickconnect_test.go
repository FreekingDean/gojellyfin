package quickconnect

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/quickconnect/mocks"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func signedIn(userID uuid.UUID) context.Context {
	return auth.ContextWithSession(context.Background(), &sessions.Session{UserID: &userID})
}

func TestServer_InitiateQuickConnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := mocks.NewMockQuickConnectService(ctrl)
	server := &Server{quickconnect: service}

	at := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	ctx := auth.ContextWithAuthorization(context.Background(), auth.Authorization{
		Client: "Jellyfin Web", Device: "tv", DeviceID: "tv-1", Version: "10.10.0",
	})

	service.EXPECT().
		Initiate(ctx, sessions.Device{ClientID: "tv-1", Name: "tv", AppName: "Jellyfin Web", AppVersion: "10.10.0"}).
		Return(&quickconnect.Request{
			Secret: "secret", Code: "123456", DeviceID: "tv-1", DeviceName: "tv",
			AppName: "Jellyfin Web", AppVersion: "10.10.0", CreatedAt: at,
		}, nil)

	response, err := server.InitiateQuickConnect(ctx, api.InitiateQuickConnectRequestObject{})

	assert.NoError(t, err)
	assert.Equal(t, api.InitiateQuickConnect200JSONResponse{
		Authenticated: apiutil.Ptr(false),
		Secret:        apiutil.Ptr("secret"),
		Code:          apiutil.Ptr("123456"),
		DeviceId:      apiutil.Ptr("tv-1"),
		DeviceName:    apiutil.Ptr("tv"),
		AppName:       apiutil.Ptr("Jellyfin Web"),
		AppVersion:    apiutil.Ptr("10.10.0"),
		DateAdded:     apiutil.Ptr(at),
	}, response)
}

func TestServer_GetQuickConnectState(t *testing.T) {
	userID := uuid.New()
	failed := errors.New("the database is gone")

	tests := []struct {
		name             string
		mockResponse     *quickconnect.Request
		mockError        error
		expectedResponse api.GetQuickConnectStateResponseObject
		expectedError    error
	}{
		{
			name:             "unknown secret",
			mockError:        quickconnect.ErrNotFound,
			expectedResponse: api.GetQuickConnectState404JSONResponse{},
		},
		{
			name:          "failure",
			mockError:     failed,
			expectedError: failed,
		},
		{
			name:         "authorized",
			mockResponse: &quickconnect.Request{Secret: "secret", AuthorizedByID: &userID},
			expectedResponse: api.GetQuickConnectState200JSONResponse{
				Authenticated: apiutil.Ptr(true),
				Secret:        apiutil.Ptr("secret"),
				Code:          apiutil.Ptr(""),
				DeviceId:      apiutil.Ptr(""),
				DeviceName:    apiutil.Ptr(""),
				AppName:       apiutil.Ptr(""),
				AppVersion:    apiutil.Ptr(""),
				DateAdded:     apiutil.Ptr(time.Time{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			service := mocks.NewMockQuickConnectService(ctrl)
			server := &Server{quickconnect: service}

			service.EXPECT().Pending(gomock.Any(), "secret").Return(tt.mockResponse, tt.mockError)

			response, err := server.GetQuickConnectState(context.Background(), api.GetQuickConnectStateRequestObject{
				Params: api.GetQuickConnectStateParams{Secret: "secret"},
			})

			assert.ErrorIs(t, err, tt.expectedError)
			assert.Equal(t, tt.expectedResponse, response)
		})
	}
}

func TestServer_AuthorizeQuickConnect(t *testing.T) {
	caller := uuid.New()
	target := uuid.New()

	tests := []struct {
		name             string
		target           *uuid.UUID
		administrator    bool
		targetError      error
		authorizeAs      uuid.UUID
		authorizeError   error
		expectedResponse api.AuthorizeQuickConnectResponseObject
	}{
		{
			name:             "authorizes for the caller",
			authorizeAs:      caller,
			expectedResponse: api.AuthorizeQuickConnect200JSONResponse(true),
		},
		{
			name:             "answers false when nothing was authorized",
			authorizeAs:      caller,
			authorizeError:   quickconnect.ErrNotFound,
			expectedResponse: api.AuthorizeQuickConnect200JSONResponse(false),
		},
		{
			name:             "refuses another user to a caller who is not an administrator",
			target:           &target,
			expectedResponse: api.AuthorizeQuickConnect403JSONResponse{},
		},
		{
			name:             "refuses a target that does not exist",
			target:           &target,
			administrator:    true,
			targetError:      errors.New("not found"),
			expectedResponse: api.AuthorizeQuickConnect403JSONResponse{},
		},
		{
			name:             "lets an administrator authorize for the target",
			target:           &target,
			administrator:    true,
			authorizeAs:      target,
			expectedResponse: api.AuthorizeQuickConnect200JSONResponse(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			service := mocks.NewMockQuickConnectService(ctrl)
			accounts := mocks.NewMockUsersService(ctrl)
			server := &Server{quickconnect: service, users: accounts}

			if tt.target != nil {
				accounts.EXPECT().IsAdministrator(gomock.Any(), caller).Return(tt.administrator, nil)
				if tt.administrator {
					accounts.EXPECT().User(gomock.Any(), target).Return(&users.User{ID: target}, tt.targetError)
				}
			}
			if tt.authorizeAs != uuid.Nil {
				service.EXPECT().Authorize(gomock.Any(), "123456", tt.authorizeAs).Return(tt.authorizeError)
			}

			response, err := server.AuthorizeQuickConnect(signedIn(caller), api.AuthorizeQuickConnectRequestObject{
				Params: api.AuthorizeQuickConnectParams{Code: "123456", UserId: tt.target},
			})

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResponse, response)
		})
	}
}
