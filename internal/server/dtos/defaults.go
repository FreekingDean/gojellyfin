package dtos

import "github.com/FreekingDean/gojellyfin/internal/server/api"

func DefaultServerConfiguration() api.ServerConfiguration {
	return api.ServerConfiguration{
		ServerName:                    Ptr("gojellyfin"),
		UICulture:                     Ptr("en-US"),
		IsStartupWizardCompleted:      Ptr(true),
		IsPortAuthorized:              Ptr(true),
		QuickConnectAvailable:         Ptr(false),
		EnableMetrics:                 Ptr(false),
		EnableFolderView:              Ptr(false),
		EnableGroupingIntoCollections: Ptr(false),
		DisplaySpecialsWithinSeasons:  Ptr(true),
		PreferredMetadataLanguage:     Ptr("en"),
		MetadataCountryCode:           Ptr("US"),
		LogFileRetentionDays:          Ptr(int32(3)),
		ActivityLogRetentionDays:      Ptr(int32(30)),
		MinResumePct:                  Ptr(int32(5)),
		MaxResumePct:                  Ptr(int32(90)),
		MinResumeDurationSeconds:      Ptr(int32(300)),
		MinAudiobookResume:            Ptr(int32(5)),
		MaxAudiobookResume:            Ptr(int32(5)),
		InactiveSessionThreshold:      Ptr(int32(0)),
		LibraryMonitorDelay:           Ptr(int32(60)),
		LibraryUpdateDuration:         Ptr(int32(30)),
		ImageExtractionTimeoutMs:      Ptr(int32(0)),
		RemoteClientBitrateLimit:      Ptr(int32(0)),
		AllowClientLogUpload:          Ptr(true),
		RemoveOldPlugins:              Ptr(false),
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
		CustomCss:           Ptr(""),
		LoginDisclaimer:     Ptr("This is a go server mimicing jellyfin dont be afraid."),
		SplashscreenEnabled: Ptr(false),
	}
}

func DefaultMetadataOptions() api.MetadataOptions {
	return api.MetadataOptions{
		ItemType:                 Ptr(""),
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
