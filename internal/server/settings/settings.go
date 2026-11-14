package settings

import (
	"context"
	"errors"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
)

type Shelves interface {
	Choices(ctx context.Context) ([]shelves.Choice, error)
	Enable(ctx context.Context, slugs []string) error
}

type Server struct {
	environment env.Config
	shelves     Shelves
}

func New(environment env.Config, shelves *shelves.Service) *Server {
	return &Server{environment: environment, shelves: shelves}
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
