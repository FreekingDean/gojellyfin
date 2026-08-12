package setup

import (
	"context"
	"encoding/json"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

const (
	startupWizardCompletedField    = "IsStartupWizardCompleted"
	uiCultureField                 = "UICulture"
	metadataCountryCodeField       = "MetadataCountryCode"
	preferredMetadataLanguageField = "PreferredMetadataLanguage"
)

type Culture struct {
	UICulture                 string
	MetadataCountryCode       string
	PreferredMetadataLanguage string
}

func DefaultCulture() Culture {
	return Culture{
		UICulture:                 "en-US",
		MetadataCountryCode:       "US",
		PreferredMetadataLanguage: "en",
	}
}

type Service struct {
	config *config.Service
	users  *users.Service
}

func New(config *config.Service, users *users.Service) *Service {
	return &Service{config: config, users: users}
}

func (s *Service) Completed(ctx context.Context) (bool, error) {
	settings, err := s.settings(ctx)
	if err != nil {
		return false, err
	}

	var completed *bool
	if raw, ok := settings[startupWizardCompletedField]; ok {
		if err := json.Unmarshal(raw, &completed); err != nil {
			return false, err
		}
	}
	if completed != nil {
		return *completed, nil
	}

	return s.users.HasUsers(ctx)
}

func (s *Service) SetCompleted(ctx context.Context, completed bool) error {
	settings, err := s.settings(ctx)
	if err != nil {
		return err
	}
	if err := set(settings, startupWizardCompletedField, completed); err != nil {
		return err
	}

	return s.save(ctx, settings)
}

func (s *Service) Culture(ctx context.Context) (Culture, error) {
	settings, err := s.settings(ctx)
	if err != nil {
		return Culture{}, err
	}

	culture := DefaultCulture()
	for field, target := range map[string]*string{
		uiCultureField:                 &culture.UICulture,
		metadataCountryCodeField:       &culture.MetadataCountryCode,
		preferredMetadataLanguageField: &culture.PreferredMetadataLanguage,
	} {
		raw, ok := settings[field]
		if !ok {
			continue
		}

		var stored *string
		if err := json.Unmarshal(raw, &stored); err != nil {
			return Culture{}, err
		}
		if stored != nil {
			*target = *stored
		}
	}

	return culture, nil
}

func (s *Service) SetCulture(ctx context.Context, culture Culture) error {
	settings, err := s.settings(ctx)
	if err != nil {
		return err
	}

	for field, value := range map[string]string{
		uiCultureField:                 culture.UICulture,
		metadataCountryCodeField:       culture.MetadataCountryCode,
		preferredMetadataLanguageField: culture.PreferredMetadataLanguage,
	} {
		if err := set(settings, field, value); err != nil {
			return err
		}
	}

	return s.save(ctx, settings)
}

// Raw fields rather than a struct: the row is written back whole, so a typed
// round trip would drop every setting this domain does not model.
func (s *Service) settings(ctx context.Context) (map[string]json.RawMessage, error) {
	value, err := s.config.Configuration(ctx, config.SystemConfigurationKey)
	if err != nil {
		return nil, err
	}

	settings := map[string]json.RawMessage{}
	if value != nil {
		if err := json.Unmarshal(value, &settings); err != nil {
			return nil, err
		}
	}

	return settings, nil
}

func (s *Service) save(ctx context.Context, settings map[string]json.RawMessage) error {
	value, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	return s.config.SetConfiguration(ctx, config.SystemConfigurationKey, value)
}

func set(settings map[string]json.RawMessage, field string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	settings[field] = encoded

	return nil
}
