package settings

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/prank"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
)

type Shelves interface {
	Choices(ctx context.Context) ([]shelves.Choice, error)
	Enable(ctx context.Context, slugs []string) error
}

type Pranks interface {
	Override(ctx context.Context) (prank.Override, error)
	Save(ctx context.Context, override prank.Override) error
	Name(ctx context.Context, override prank.Override) (string, error)
}

type Server struct {
	environment env.Config
	shelves     Shelves
	pranks      Pranks
}

func New(environment env.Config, shelves *shelves.Service, pranks *prank.Service) *Server {
	return &Server{environment: environment, shelves: shelves, pranks: pranks}
}

func (s *Server) GoJellyfinListSettings(
	_ context.Context,
	_ api.GoJellyfinListSettingsRequestObject,
) (api.GoJellyfinListSettingsResponseObject, error) {
	variables := s.environment.Variables()

	listed := make(api.GoJellyfinListSettings200JSONResponse, len(variables))
	for i, variable := range variables {
		listed[i] = api.Setting{Name: variable.Name, Value: variable.Value, Set: variable.Set, Secret: variable.Secret}
	}

	return listed, nil
}

func (s *Server) GoJellyfinListShelves(
	ctx context.Context,
	_ api.GoJellyfinListShelvesRequestObject,
) (api.GoJellyfinListShelvesResponseObject, error) {
	choices, err := s.shelves.Choices(ctx)
	if err != nil {
		return nil, err
	}

	listed := make(api.GoJellyfinListShelves200JSONResponse, len(choices))
	for i, choice := range choices {
		listed[i] = api.ShelfChoice{Slug: choice.Slug, Name: choice.Name, Enabled: choice.Enabled}
	}

	return listed, nil
}

func (s *Server) GoJellyfinUpdateShelves(
	ctx context.Context,
	request api.GoJellyfinUpdateShelvesRequestObject,
) (api.GoJellyfinUpdateShelvesResponseObject, error) {
	if request.Body == nil {
		return api.GoJellyfinUpdateShelves400JSONResponse{Message: "no request body"}, nil
	}

	err := s.shelves.Enable(ctx, request.Body.Enabled)
	switch {
	case errors.Is(err, shelves.ErrUnknown):
		return api.GoJellyfinUpdateShelves400JSONResponse{Message: err.Error()}, nil
	case err != nil:
		return nil, err
	}

	return api.GoJellyfinUpdateShelves204Response{}, nil
}

func (s *Server) GoJellyfinGetPlaybackOverride(
	ctx context.Context,
	_ api.GoJellyfinGetPlaybackOverrideRequestObject,
) (api.GoJellyfinGetPlaybackOverrideResponseObject, error) {
	override, err := s.pranks.Override(ctx)
	if err != nil {
		return nil, err
	}

	name, err := s.pranks.Name(ctx, override)
	if err != nil {
		return nil, err
	}

	answer := api.PlaybackOverride{
		Enabled: override.Enabled,
		ItemId:  override.ItemID,
		From:    override.From,
		Until:   override.Until,
		UserIds: override.UserIDs,
	}
	if answer.UserIds == nil {
		answer.UserIds = []uuid.UUID{}
	}
	if name != "" {
		answer.ItemName = &name
	}

	return api.GoJellyfinGetPlaybackOverride200JSONResponse(answer), nil
}

func (s *Server) GoJellyfinUpdatePlaybackOverride(
	ctx context.Context,
	request api.GoJellyfinUpdatePlaybackOverrideRequestObject,
) (api.GoJellyfinUpdatePlaybackOverrideResponseObject, error) {
	if request.Body == nil {
		return api.GoJellyfinUpdatePlaybackOverride400JSONResponse{Message: "no request body"}, nil
	}

	err := s.pranks.Save(ctx, prank.Override{
		Enabled: request.Body.Enabled,
		ItemID:  request.Body.ItemId,
		From:    request.Body.From,
		Until:   request.Body.Until,
		UserIDs: request.Body.UserIds,
	})
	switch {
	case errors.Is(err, prank.ErrInvalid):
		return api.GoJellyfinUpdatePlaybackOverride400JSONResponse{Message: err.Error()}, nil
	case err != nil:
		return nil, err
	}

	return api.GoJellyfinUpdatePlaybackOverride204Response{}, nil
}
