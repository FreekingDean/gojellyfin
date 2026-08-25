package displaypreferences

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/google/uuid"
)

type Server struct {
	preferences *displaypreferences.Service
}

func New(preferences *displaypreferences.Service) *Server {
	return &Server{preferences: preferences}
}

func (s *Server) GetDisplayPreferences(ctx context.Context, request api.GetDisplayPreferencesRequestObject) (api.GetDisplayPreferencesResponseObject, error) {
	userID := apiutil.OrElse(request.Params.UserId, auth.UserID(ctx))
	refID := md5(request.DisplayPreferencesId)

	prefs, err := s.preferences.Get(ctx, userID, refID, request.Params.Client)
	if err != nil {
		return nil, err
	}

	return api.GetDisplayPreferences200JSONResponse(displayPreferencesDto(request.DisplayPreferencesId, prefs)), nil
}

func (s *Server) UpdateDisplayPreferences(ctx context.Context, request api.UpdateDisplayPreferencesRequestObject) (api.UpdateDisplayPreferencesResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateDisplayPreferences204Response{}, nil
	}

	userID := apiutil.OrElse(request.Params.UserId, auth.UserID(ctx))
	refID := md5(request.DisplayPreferencesId)

	if err := s.preferences.Update(ctx, userID, refID, request.Params.Client, settings(req)); err != nil {
		return nil, err
	}

	return api.UpdateDisplayPreferences204Response{}, nil
}

func md5(s string) uuid.UUID {
	return uuid.NewMD5(uuid.New(), []byte(s))
}
