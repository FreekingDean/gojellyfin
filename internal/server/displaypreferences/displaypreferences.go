package displaypreferences

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	preferences *displaypreferences.Service
}

func New(preferences *displaypreferences.Service) *Server {
	return &Server{preferences: preferences}
}

func (s *Server) GetDisplayPreferences(ctx context.Context, request api.GetDisplayPreferencesRequestObject) (api.GetDisplayPreferencesResponseObject, error) {
	userID := apiutil.Deref(apiutil.OrElse(request.Params.UserId, auth.UserID(ctx)))
	prefs, err := s.preferences.Preferences(ctx, userID, request.Params.Client, request.DisplayPreferencesId)
	if err != nil {
		return nil, err
	}

	return api.GetDisplayPreferences200JSONResponse(DisplayPreferencesDto(request.DisplayPreferencesId, prefs)), nil
}

func (s *Server) UpdateDisplayPreferences(ctx context.Context, request api.UpdateDisplayPreferencesRequestObject) (api.UpdateDisplayPreferencesResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateDisplayPreferences204Response{}, nil
	}

	userID := apiutil.Deref(apiutil.OrElse(request.Params.UserId, auth.UserID(ctx)))
	prefs, err := s.preferences.Preferences(ctx, userID, request.Params.Client, request.DisplayPreferencesId)
	if err != nil {
		return nil, err
	}

	if _, err := s.preferences.UpdateSettings(ctx, prefs.ID, Settings(req)); err != nil {
		return nil, err
	}

	return api.UpdateDisplayPreferences204Response{}, nil
}
