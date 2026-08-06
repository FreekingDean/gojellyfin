package configuration

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	config *config.Service
}

func New(config *config.Service) *Server {
	return &Server{config: config}
}

func (s *Server) GetConfiguration(ctx context.Context, request api.GetConfigurationRequestObject) (api.GetConfigurationResponseObject, error) {
	configuration, err := ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	return api.GetConfiguration200JSONResponse(configuration), nil
}

func (s *Server) UpdateConfiguration(ctx context.Context, request api.UpdateConfigurationRequestObject) (api.UpdateConfigurationResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateConfiguration403Response{}, nil
	}

	value, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := s.config.SetConfiguration(ctx, SystemConfigurationKey, value); err != nil {
		return nil, err
	}

	return api.UpdateConfiguration204Response{}, nil
}

func (s *Server) GetNamedConfiguration(ctx context.Context, request api.GetNamedConfigurationRequestObject) (api.GetNamedConfigurationResponseObject, error) {
	key := strings.ToLower(request.Key)

	value, err := s.config.Configuration(ctx, key)
	if err != nil {
		return nil, err
	}
	if value == nil {
		if value, err = json.Marshal(DefaultNamedConfiguration(key)); err != nil {
			return nil, err
		}
	}

	var configuration map[string]any
	if err := json.Unmarshal(value, &configuration); err != nil {
		return nil, err
	}

	return api.GetNamedConfiguration200JSONResponse(configuration), nil
}

func (s *Server) UpdateNamedConfiguration(ctx context.Context, request api.UpdateNamedConfigurationRequestObject) (api.UpdateNamedConfigurationResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateNamedConfiguration403Response{}, nil
	}

	value, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := s.config.SetConfiguration(ctx, strings.ToLower(request.Key), value); err != nil {
		return nil, err
	}

	return api.UpdateNamedConfiguration204Response{}, nil
}

func (s *Server) GetDefaultMetadataOptions(ctx context.Context, request api.GetDefaultMetadataOptionsRequestObject) (api.GetDefaultMetadataOptionsResponseObject, error) {
	return api.GetDefaultMetadataOptions200JSONResponse(DefaultMetadataOptions()), nil
}
