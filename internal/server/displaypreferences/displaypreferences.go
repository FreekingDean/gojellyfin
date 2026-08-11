package displaypreferences

import (
	"context"

	"github.com/google/uuid"

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
	prefs, err := s.userPreferences(ctx, request.Params.Client, request.DisplayPreferencesId)
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

	prefs, err := s.userPreferences(ctx, request.Params.Client, request.DisplayPreferencesId)
	if err != nil {
		return nil, err
	}

	update := s.preferences.Update(prefs.ID).
		SetNillableViewType(req.ViewType).
		SetNillableSortBy(req.SortBy).
		SetNillableIndexBy(req.IndexBy).
		SetNillableRememberIndexing(req.RememberIndexing).
		SetNillableRememberSorting(req.RememberSorting).
		SetNillableShowBackdrop(req.ShowBackdrop).
		SetNillableShowSidebar(req.ShowSidebar).
		SetNillablePrimaryImageHeight(req.PrimaryImageHeight).
		SetNillablePrimaryImageWidth(req.PrimaryImageWidth)

	if req.SortOrder != nil {
		update.SetSortOrder(displaypreferences.SortOrder(*req.SortOrder))
	}
	if req.ScrollDirection != nil {
		update.SetScrollDirection(displaypreferences.ScrollDirection(*req.ScrollDirection))
	}
	if req.CustomPrefs != nil {
		update.SetCustomPrefs(customPrefs(*req.CustomPrefs))
	}

	if err := update.Exec(ctx); err != nil {
		return nil, err
	}

	return api.UpdateDisplayPreferences204Response{}, nil
}

func (s *Server) userPreferences(ctx context.Context, client, id string) (*displaypreferences.DisplayPreferences, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return nil, auth.ErrUnauthorized
	}

	return s.preferences.Preferences(ctx, userID, client, id)
}
