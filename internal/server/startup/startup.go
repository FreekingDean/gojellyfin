package startup

import (
	"context"
	"encoding/json"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/setup"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	config *config.Service
	setup  *setup.Service
	users  *users.Service
}

func New(config *config.Service, setup *setup.Service, users *users.Service) *Server {
	return &Server{config: config, setup: setup, users: users}
}

func (s *Server) GetStartupConfiguration(
	ctx context.Context,
	request api.GetStartupConfigurationRequestObject,
) (api.GetStartupConfigurationResponseObject, error) {
	completed, err := s.setup.Completed(ctx)
	if err != nil {
		return nil, err
	}
	if completed {
		return api.GetStartupConfiguration403Response{}, nil
	}

	culture, err := s.setup.Culture(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetStartupConfiguration200JSONResponse(startupConfiguration(culture)), nil
}

func (s *Server) UpdateInitialConfiguration(
	ctx context.Context,
	request api.UpdateInitialConfigurationRequestObject,
) (api.UpdateInitialConfigurationResponseObject, error) {
	completed, err := s.setup.Completed(ctx)
	if err != nil {
		return nil, err
	}
	if completed {
		return api.UpdateInitialConfiguration403Response{}, nil
	}

	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateInitialConfiguration403Response{}, nil
	}

	culture, err := s.setup.Culture(ctx)
	if err != nil {
		return nil, err
	}
	if req.UICulture != nil {
		culture.UICulture = *req.UICulture
	}
	if req.MetadataCountryCode != nil {
		culture.MetadataCountryCode = *req.MetadataCountryCode
	}
	if req.PreferredMetadataLanguage != nil {
		culture.PreferredMetadataLanguage = *req.PreferredMetadataLanguage
	}

	if err := s.setup.SetCulture(ctx, culture); err != nil {
		return nil, err
	}
	if err := s.keepWizardOpen(ctx); err != nil {
		return nil, err
	}

	return api.UpdateInitialConfiguration204Response{}, nil
}

func (s *Server) GetFirstUser(
	ctx context.Context,
	request api.GetFirstUserRequestObject,
) (api.GetFirstUserResponseObject, error) {
	completed, err := s.setup.Completed(ctx)
	if err != nil {
		return nil, err
	}
	if completed {
		return api.GetFirstUser403Response{}, nil
	}

	user, err := s.users.FirstUser(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetFirstUser200JSONResponse(startupUser(user)), nil
}

func (s *Server) GetFirstUser2(
	ctx context.Context,
	request api.GetFirstUser2RequestObject,
) (api.GetFirstUser2ResponseObject, error) {
	completed, err := s.setup.Completed(ctx)
	if err != nil {
		return nil, err
	}
	if completed {
		return api.GetFirstUser2403Response{}, nil
	}

	user, err := s.users.FirstUser(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetFirstUser2200JSONResponse(startupUser(user)), nil
}

func (s *Server) UpdateStartupUser(
	ctx context.Context,
	request api.UpdateStartupUserRequestObject,
) (api.UpdateStartupUserResponseObject, error) {
	completed, err := s.setup.Completed(ctx)
	if err != nil {
		return nil, err
	}
	if completed {
		return api.UpdateStartupUser403Response{}, nil
	}

	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || apiutil.Deref(req.Name) == "" {
		return api.UpdateStartupUser403Response{}, nil
	}

	hash, err := auth.Hash(apiutil.Deref(req.Password))
	if err != nil {
		return nil, err
	}

	if err := s.keepWizardOpen(ctx); err != nil {
		return nil, err
	}
	if _, err := s.users.EnsureUser(ctx, *req.Name, hash, true); err != nil {
		return nil, err
	}

	return api.UpdateStartupUser204Response{}, nil
}

func (s *Server) SetRemoteAccess(
	ctx context.Context,
	request api.SetRemoteAccessRequestObject,
) (api.SetRemoteAccessResponseObject, error) {
	completed, err := s.setup.Completed(ctx)
	if err != nil {
		return nil, err
	}
	if completed {
		return api.SetRemoteAccess403Response{}, nil
	}

	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.SetRemoteAccess403Response{}, nil
	}

	value, err := json.Marshal(networkConfiguration{
		EnableRemoteAccess: req.EnableRemoteAccess,
		EnableUPnP:         req.EnableAutomaticPortMapping,
	})
	if err != nil {
		return nil, err
	}
	if err := s.config.SetConfiguration(ctx, networkConfigurationKey, value); err != nil {
		return nil, err
	}

	return api.SetRemoteAccess204Response{}, nil
}

func (s *Server) CompleteWizard(
	ctx context.Context,
	request api.CompleteWizardRequestObject,
) (api.CompleteWizardResponseObject, error) {
	completed, err := s.setup.Completed(ctx)
	if err != nil {
		return nil, err
	}
	if completed {
		return api.CompleteWizard403Response{}, nil
	}

	if err := s.setup.SetCompleted(ctx, true); err != nil {
		return nil, err
	}

	return api.CompleteWizard204Response{}, nil
}

func (s *Server) keepWizardOpen(ctx context.Context) error {
	return s.setup.SetCompleted(ctx, false)
}
