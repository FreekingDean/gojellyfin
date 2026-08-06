package configuration

import (
	"context"
	"encoding/json"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
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

func DefaultServerConfiguration() api.ServerConfiguration {
	return api.ServerConfiguration{
		ServerName:                    apiutil.Ptr("gojellyfin"),
		UICulture:                     apiutil.Ptr("en-US"),
		IsStartupWizardCompleted:      apiutil.Ptr(true),
		IsPortAuthorized:              apiutil.Ptr(true),
		QuickConnectAvailable:         apiutil.Ptr(false),
		EnableMetrics:                 apiutil.Ptr(false),
		EnableFolderView:              apiutil.Ptr(false),
		EnableGroupingIntoCollections: apiutil.Ptr(false),
		DisplaySpecialsWithinSeasons:  apiutil.Ptr(true),
		PreferredMetadataLanguage:     apiutil.Ptr("en"),
		MetadataCountryCode:           apiutil.Ptr("US"),
		LogFileRetentionDays:          apiutil.Ptr(int32(3)),
		ActivityLogRetentionDays:      apiutil.Ptr(int32(30)),
		MinResumePct:                  apiutil.Ptr(int32(5)),
		MaxResumePct:                  apiutil.Ptr(int32(90)),
		MinResumeDurationSeconds:      apiutil.Ptr(int32(300)),
		MinAudiobookResume:            apiutil.Ptr(int32(5)),
		MaxAudiobookResume:            apiutil.Ptr(int32(5)),
		InactiveSessionThreshold:      apiutil.Ptr(int32(0)),
		LibraryMonitorDelay:           apiutil.Ptr(int32(60)),
		LibraryUpdateDuration:         apiutil.Ptr(int32(30)),
		ImageExtractionTimeoutMs:      apiutil.Ptr(int32(0)),
		RemoteClientBitrateLimit:      apiutil.Ptr(int32(0)),
		AllowClientLogUpload:          apiutil.Ptr(true),
		RemoveOldPlugins:              apiutil.Ptr(false),
		SortReplaceCharacters:         &[]string{".", "+", "%"},
		SortRemoveCharacters:          &[]string{",", "&", "-", "{", "}", "'"},
		SortRemoveWords:               &[]string{"the", "a", "an"},
		CorsHosts:                     &[]string{"*"},
		CodecsUsed:                    &[]string{},
		PluginRepositories:            &[]api.RepositoryInfo{},
		ContentTypes:                  &[]api.NameValuePair{},
		PathSubstitutions:             &[]api.PathSubstitution{},
		MetadataOptions:               &[]api.MetadataOptions{DefaultMetadataOptions()},
		CastReceiverApplications:      &[]api.CastReceiverApplication{},
	}
}

func DefaultBrandingOptions() api.BrandingOptions {
	return api.BrandingOptions{
		CustomCss:           apiutil.Ptr(""),
		LoginDisclaimer:     apiutil.Ptr("This is a go server mimicing jellyfin dont be afraid."),
		SplashscreenEnabled: apiutil.Ptr(false),
	}
}

func DefaultMetadataOptions() api.MetadataOptions {
	return api.MetadataOptions{
		ItemType:                 apiutil.Ptr(""),
		DisabledMetadataFetchers: &[]string{},
		DisabledMetadataSavers:   &[]string{},
		DisabledImageFetchers:    &[]string{},
		LocalMetadataReaderOrder: &[]string{},
		MetadataFetcherOrder:     &[]string{},
		ImageFetcherOrder:        &[]string{},
	}
}

func DefaultNamedConfiguration(key string) any {
	switch key {
	case BrandingConfigurationKey:
		return DefaultBrandingOptions()
	case "metadata":
		return DefaultMetadataOptions()
	default:
		return DefaultServerConfiguration()
	}
}
