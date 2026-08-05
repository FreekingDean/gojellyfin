package server

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

const (
	systemConfigurationKey   = "system"
	brandingConfigurationKey = "branding"
)

func (s *Server) GetConfiguration(ctx context.Context, request api.GetConfigurationRequestObject) (api.GetConfigurationResponseObject, error) {
	configuration, err := s.serverConfiguration(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetConfiguration200JSONResponse(configuration), nil
}

func (s *Server) UpdateConfiguration(ctx context.Context, request api.UpdateConfigurationRequestObject) (api.UpdateConfigurationResponseObject, error) {
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateConfiguration403Response{}, nil
	}

	value, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetConfiguration(ctx, systemConfigurationKey, value); err != nil {
		return nil, err
	}

	return api.UpdateConfiguration204Response{}, nil
}

func (s *Server) GetNamedConfiguration(ctx context.Context, request api.GetNamedConfigurationRequestObject) (api.GetNamedConfigurationResponseObject, error) {
	key := strings.ToLower(request.Key)

	value, err := s.store.GetConfiguration(ctx, key)
	if err != nil {
		return nil, err
	}
	if value == nil {
		if value, err = json.Marshal(defaultNamedConfiguration(key)); err != nil {
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
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateNamedConfiguration403Response{}, nil
	}

	value, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetConfiguration(ctx, strings.ToLower(request.Key), value); err != nil {
		return nil, err
	}

	return api.UpdateNamedConfiguration204Response{}, nil
}

func (s *Server) GetDefaultMetadataOptions(ctx context.Context, request api.GetDefaultMetadataOptionsRequestObject) (api.GetDefaultMetadataOptionsResponseObject, error) {
	return api.GetDefaultMetadataOptions200JSONResponse(defaultMetadataOptions()), nil
}

func (s *Server) serverConfiguration(ctx context.Context) (api.ServerConfiguration, error) {
	configuration := defaultServerConfiguration()

	value, err := s.store.GetConfiguration(ctx, systemConfigurationKey)
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

func (s *Server) brandingConfiguration(ctx context.Context) (api.BrandingOptions, error) {
	branding := defaultBrandingOptions()

	value, err := s.store.GetConfiguration(ctx, brandingConfigurationKey)
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

func defaultNamedConfiguration(key string) any {
	switch key {
	case brandingConfigurationKey:
		return defaultBrandingOptions()
	case "metadata":
		return defaultMetadataOptions()
	default:
		return defaultServerConfiguration()
	}
}

func defaultServerConfiguration() api.ServerConfiguration {
	return api.ServerConfiguration{
		ServerName:                    ptr("gojellyfin"),
		UICulture:                     ptr("en-US"),
		IsStartupWizardCompleted:      ptr(true),
		IsPortAuthorized:              ptr(true),
		QuickConnectAvailable:         ptr(false),
		EnableMetrics:                 ptr(false),
		EnableFolderView:              ptr(false),
		EnableGroupingIntoCollections: ptr(false),
		DisplaySpecialsWithinSeasons:  ptr(true),
		PreferredMetadataLanguage:     ptr("en"),
		MetadataCountryCode:           ptr("US"),
		LogFileRetentionDays:          ptr(int32(3)),
		ActivityLogRetentionDays:      ptr(int32(30)),
		MinResumePct:                  ptr(int32(5)),
		MaxResumePct:                  ptr(int32(90)),
		MinResumeDurationSeconds:      ptr(int32(300)),
		MinAudiobookResume:            ptr(int32(5)),
		MaxAudiobookResume:            ptr(int32(5)),
		InactiveSessionThreshold:      ptr(int32(0)),
		LibraryMonitorDelay:           ptr(int32(60)),
		LibraryUpdateDuration:         ptr(int32(30)),
		ImageExtractionTimeoutMs:      ptr(int32(0)),
		RemoteClientBitrateLimit:      ptr(int32(0)),
		AllowClientLogUpload:          ptr(true),
		RemoveOldPlugins:              ptr(false),
		SortReplaceCharacters:         &[]string{".", "+", "%"},
		SortRemoveCharacters:          &[]string{",", "&", "-", "{", "}", "'"},
		SortRemoveWords:               &[]string{"the", "a", "an"},
		CorsHosts:                     &[]string{"*"},
		CodecsUsed:                    &[]string{},
		PluginRepositories:            &[]api.RepositoryInfo{},
		ContentTypes:                  &[]api.NameValuePair{},
		PathSubstitutions:             &[]api.PathSubstitution{},
		MetadataOptions:               &[]api.MetadataOptions{defaultMetadataOptions()},
		CastReceiverApplications:      &[]api.CastReceiverApplication{},
	}
}

func defaultBrandingOptions() api.BrandingOptions {
	return api.BrandingOptions{
		CustomCss:           ptr(""),
		LoginDisclaimer:     ptr("This is a go server mimicing jellyfin dont be afraid."),
		SplashscreenEnabled: ptr(false),
	}
}

func defaultMetadataOptions() api.MetadataOptions {
	return api.MetadataOptions{
		ItemType:                 ptr(""),
		DisabledMetadataFetchers: &[]string{},
		DisabledMetadataSavers:   &[]string{},
		DisabledImageFetchers:    &[]string{},
		LocalMetadataReaderOrder: &[]string{},
		MetadataFetcherOrder:     &[]string{},
		ImageFetcherOrder:        &[]string{},
	}
}
