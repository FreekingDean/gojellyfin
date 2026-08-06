package dtos

import (
	"context"
	"encoding/json"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

const (
	SystemConfigurationKey   = "system"
	BrandingConfigurationKey = "branding"
)

// Read helpers rather than methods, so any tag's handlers can use them.
func ServerConfiguration(ctx context.Context, store *config.Service) (api.ServerConfiguration, error) {
	configuration := DefaultServerConfiguration()

	value, err := store.Configuration(ctx, SystemConfigurationKey)
	if err != nil {
		return api.ServerConfiguration{}, err
	}
	if value != nil {
		if err := json.Unmarshal(value, &configuration); err != nil {
			return api.ServerConfiguration{}, err
		}
	}

	return configuration, nil
}

func BrandingConfiguration(ctx context.Context, store *config.Service) (api.BrandingOptions, error) {
	branding := DefaultBrandingOptions()

	value, err := store.Configuration(ctx, BrandingConfigurationKey)
	if err != nil {
		return api.BrandingOptions{}, err
	}
	if value != nil {
		if err := json.Unmarshal(value, &branding); err != nil {
			return api.BrandingOptions{}, err
		}
	}

	return branding, nil
}
