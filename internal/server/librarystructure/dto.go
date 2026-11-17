package librarystructure

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func virtualFolderInfo(library *libraries.Library) api.VirtualFolderInfo {
	options := optionsDto(library.Options)

	pathInfos := make([]api.MediaPathInfo, 0, len(library.Locations))
	for _, location := range library.Locations {
		pathInfos = append(pathInfos, api.MediaPathInfo{Path: apiutil.Ptr(location)})
	}
	options.PathInfos = &pathInfos

	collectionType := api.CollectionTypeOptions(library.CollectionType)

	return api.VirtualFolderInfo{
		Name:           apiutil.Ptr(library.Name),
		ItemId:         apiutil.Ptr(library.ID.String()),
		Locations:      &library.Locations,
		CollectionType: &collectionType,
		LibraryOptions: &options,
		RefreshStatus:  apiutil.Ptr("Idle"),
	}
}

func optionsDto(options *libraries.Options) api.LibraryOptions {
	if options == nil {
		return api.LibraryOptions{}
	}

	typeOptions := make([]api.TypeOptions, 0, len(options.TypeOptions))
	for _, option := range options.TypeOptions {
		typeOptions = append(typeOptions, api.TypeOptions{
			Type:                 apiutil.Ptr(option.Type),
			MetadataFetchers:     apiutil.Ptr(option.MetadataFetchers),
			MetadataFetcherOrder: apiutil.Ptr(option.MetadataFetcherOrder),
			ImageFetchers:        apiutil.Ptr(option.ImageFetchers),
			ImageFetcherOrder:    apiutil.Ptr(option.ImageFetcherOrder),
		})
	}

	return api.LibraryOptions{
		Enabled:                                 apiutil.Ptr(options.Enabled),
		EnablePhotos:                            apiutil.Ptr(options.EnablePhotos),
		EnableRealtimeMonitor:                   apiutil.Ptr(options.EnableRealtimeMonitor),
		EnableLUFSScan:                          apiutil.Ptr(options.EnableLufsScan),
		EnableChapterImageExtraction:            apiutil.Ptr(options.EnableChapterImageExtraction),
		ExtractChapterImagesDuringLibraryScan:   apiutil.Ptr(options.ExtractChapterImagesDuringLibraryScan),
		EnableTrickplayImageExtraction:          apiutil.Ptr(options.EnableTrickplayImageExtraction),
		ExtractTrickplayImagesDuringLibraryScan: apiutil.Ptr(options.ExtractTrickplayImagesDuringLibraryScan),
		SaveLocalMetadata:                       apiutil.Ptr(options.SaveLocalMetadata),
		EnableInternetProviders:                 apiutil.Ptr(options.EnableInternetProviders),
		EnableAutomaticSeriesGrouping:           apiutil.Ptr(options.EnableAutomaticSeriesGrouping),
		EnableEmbeddedTitles:                    apiutil.Ptr(options.EnableEmbeddedTitles),
		EnableEmbeddedExtrasTitles:              apiutil.Ptr(options.EnableEmbeddedExtrasTitles),
		EnableEmbeddedEpisodeInfos:              apiutil.Ptr(options.EnableEmbeddedEpisodeInfos),
		SkipSubtitlesIfEmbeddedSubtitlesPresent: apiutil.Ptr(options.SkipSubtitlesIfEmbeddedSubtitlesPresent),
		SkipSubtitlesIfAudioTrackMatches:        apiutil.Ptr(options.SkipSubtitlesIfAudioTrackMatches),
		RequirePerfectSubtitleMatch:             apiutil.Ptr(options.RequirePerfectSubtitleMatch),
		SaveSubtitlesWithMedia:                  apiutil.Ptr(options.SaveSubtitlesWithMedia),
		SaveLyricsWithMedia:                     apiutil.Ptr(options.SaveLyricsWithMedia),
		SaveTrickplayWithMedia:                  apiutil.Ptr(options.SaveTrickplayWithMedia),
		PreferNonstandardArtistsTag:             apiutil.Ptr(options.PreferNonstandardArtistsTag),
		UseCustomTagDelimiters:                  apiutil.Ptr(options.UseCustomTagDelimiters),
		AutomaticallyAddToCollection:            apiutil.Ptr(options.AutomaticallyAddToCollection),
		AutomaticRefreshIntervalDays:            apiutil.Ptr(options.AutomaticRefreshIntervalDays),
		PreferredMetadataLanguage:               apiutil.Ptr(options.PreferredMetadataLanguage),
		MetadataCountryCode:                     apiutil.Ptr(options.MetadataCountryCode),
		SeasonZeroDisplayName:                   apiutil.Ptr(options.SeasonZeroDisplayName),
		AllowEmbeddedSubtitles:                  apiutil.Ptr(api.EmbeddedSubtitleOptions(options.AllowEmbeddedSubtitles)),
		MetadataSavers:                          &options.MetadataSavers,
		DisabledLocalMetadataReaders:            &options.DisabledLocalMetadataReaders,
		LocalMetadataReaderOrder:                &options.LocalMetadataReaderOrder,
		DisabledSubtitleFetchers:                &options.DisabledSubtitleFetchers,
		SubtitleFetcherOrder:                    &options.SubtitleFetcherOrder,
		DisabledMediaSegmentProviders:           &options.DisabledMediaSegmentProviders,
		MediaSegmentProviderOrder:               &options.MediaSegmentProviderOrder,
		SubtitleDownloadLanguages:               &options.SubtitleDownloadLanguages,
		DisabledLyricFetchers:                   &options.DisabledLyricFetchers,
		LyricFetcherOrder:                       &options.LyricFetcherOrder,
		CustomTagDelimiters:                     &options.CustomTagDelimiters,
		DelimiterWhitelist:                      &options.DelimiterWhitelist,
		TypeOptions:                             &typeOptions,
		PathInfos:                               &[]api.MediaPathInfo{},
	}
}

func (s *Server) saveOptions(ctx context.Context, id uuid.UUID, req *api.LibraryOptions) error {
	if req == nil {
		return nil
	}

	return s.libraries.SaveOptions(ctx, id, func(options *libraries.Options) {
		mergeOptions(options, req)
	})
}

func mergeOptions(options *libraries.Options, req *api.LibraryOptions) {
	mergeOrders(options, req)
	options.Enabled = apiutil.OrElse(req.Enabled, options.Enabled)
	options.EnablePhotos = apiutil.OrElse(req.EnablePhotos, options.EnablePhotos)
	options.EnableRealtimeMonitor = apiutil.OrElse(req.EnableRealtimeMonitor, options.EnableRealtimeMonitor)
	options.EnableLufsScan = apiutil.OrElse(req.EnableLUFSScan, options.EnableLufsScan)
	options.EnableChapterImageExtraction = apiutil.OrElse(req.EnableChapterImageExtraction, options.EnableChapterImageExtraction)
	options.ExtractChapterImagesDuringLibraryScan = apiutil.OrElse(req.ExtractChapterImagesDuringLibraryScan, options.ExtractChapterImagesDuringLibraryScan)
	options.EnableTrickplayImageExtraction = apiutil.OrElse(req.EnableTrickplayImageExtraction, options.EnableTrickplayImageExtraction)
	options.ExtractTrickplayImagesDuringLibraryScan = apiutil.OrElse(req.ExtractTrickplayImagesDuringLibraryScan, options.ExtractTrickplayImagesDuringLibraryScan)
	options.SaveLocalMetadata = apiutil.OrElse(req.SaveLocalMetadata, options.SaveLocalMetadata)
	options.EnableInternetProviders = apiutil.OrElse(req.EnableInternetProviders, options.EnableInternetProviders) //nolint:staticcheck
	options.EnableAutomaticSeriesGrouping = apiutil.OrElse(req.EnableAutomaticSeriesGrouping, options.EnableAutomaticSeriesGrouping)
	options.EnableEmbeddedTitles = apiutil.OrElse(req.EnableEmbeddedTitles, options.EnableEmbeddedTitles)
	options.EnableEmbeddedExtrasTitles = apiutil.OrElse(req.EnableEmbeddedExtrasTitles, options.EnableEmbeddedExtrasTitles)
	options.EnableEmbeddedEpisodeInfos = apiutil.OrElse(req.EnableEmbeddedEpisodeInfos, options.EnableEmbeddedEpisodeInfos)
	options.SkipSubtitlesIfEmbeddedSubtitlesPresent = apiutil.OrElse(req.SkipSubtitlesIfEmbeddedSubtitlesPresent, options.SkipSubtitlesIfEmbeddedSubtitlesPresent)
	options.SkipSubtitlesIfAudioTrackMatches = apiutil.OrElse(req.SkipSubtitlesIfAudioTrackMatches, options.SkipSubtitlesIfAudioTrackMatches)
	options.RequirePerfectSubtitleMatch = apiutil.OrElse(req.RequirePerfectSubtitleMatch, options.RequirePerfectSubtitleMatch)
	options.SaveSubtitlesWithMedia = apiutil.OrElse(req.SaveSubtitlesWithMedia, options.SaveSubtitlesWithMedia)
	options.SaveLyricsWithMedia = apiutil.OrElse(req.SaveLyricsWithMedia, options.SaveLyricsWithMedia)
	options.SaveTrickplayWithMedia = apiutil.OrElse(req.SaveTrickplayWithMedia, options.SaveTrickplayWithMedia)
	options.PreferNonstandardArtistsTag = apiutil.OrElse(req.PreferNonstandardArtistsTag, options.PreferNonstandardArtistsTag)
	options.UseCustomTagDelimiters = apiutil.OrElse(req.UseCustomTagDelimiters, options.UseCustomTagDelimiters)
	options.AutomaticallyAddToCollection = apiutil.OrElse(req.AutomaticallyAddToCollection, options.AutomaticallyAddToCollection)
	options.AutomaticRefreshIntervalDays = apiutil.OrElse(req.AutomaticRefreshIntervalDays, options.AutomaticRefreshIntervalDays)
	options.PreferredMetadataLanguage = apiutil.OrElse(req.PreferredMetadataLanguage, options.PreferredMetadataLanguage)
	options.MetadataCountryCode = apiutil.OrElse(req.MetadataCountryCode, options.MetadataCountryCode)
	options.SeasonZeroDisplayName = apiutil.OrElse(req.SeasonZeroDisplayName, options.SeasonZeroDisplayName)
	if req.AllowEmbeddedSubtitles != nil {
		options.AllowEmbeddedSubtitles = libraries.EmbeddedSubtitles(*req.AllowEmbeddedSubtitles)
	}
	if req.TypeOptions != nil {
		options.TypeOptions = typeOptions(*req.TypeOptions)
	}
}

func mergeOrders(options *libraries.Options, req *api.LibraryOptions) {
	options.MetadataSavers = apiutil.OrElse(req.MetadataSavers, options.MetadataSavers)
	options.DisabledLocalMetadataReaders = apiutil.OrElse(req.DisabledLocalMetadataReaders, options.DisabledLocalMetadataReaders)
	options.LocalMetadataReaderOrder = apiutil.OrElse(req.LocalMetadataReaderOrder, options.LocalMetadataReaderOrder)
	options.DisabledSubtitleFetchers = apiutil.OrElse(req.DisabledSubtitleFetchers, options.DisabledSubtitleFetchers)
	options.SubtitleFetcherOrder = apiutil.OrElse(req.SubtitleFetcherOrder, options.SubtitleFetcherOrder)
	options.DisabledMediaSegmentProviders = apiutil.OrElse(req.DisabledMediaSegmentProviders, options.DisabledMediaSegmentProviders)
	options.MediaSegmentProviderOrder = apiutil.OrElse(req.MediaSegmentProviderOrder, options.MediaSegmentProviderOrder)
	options.SubtitleDownloadLanguages = apiutil.OrElse(req.SubtitleDownloadLanguages, options.SubtitleDownloadLanguages)
	options.DisabledLyricFetchers = apiutil.OrElse(req.DisabledLyricFetchers, options.DisabledLyricFetchers)
	options.LyricFetcherOrder = apiutil.OrElse(req.LyricFetcherOrder, options.LyricFetcherOrder)
	options.CustomTagDelimiters = apiutil.OrElse(req.CustomTagDelimiters, options.CustomTagDelimiters)
	options.DelimiterWhitelist = apiutil.OrElse(req.DelimiterWhitelist, options.DelimiterWhitelist)
}

func typeOptions(requested []api.TypeOptions) []libraries.TypeOptions {
	options := make([]libraries.TypeOptions, 0, len(requested))
	for _, option := range requested {
		options = append(options, libraries.TypeOptions{
			Type:                 apiutil.Deref(option.Type),
			MetadataFetchers:     apiutil.Deref(option.MetadataFetchers),
			MetadataFetcherOrder: apiutil.Deref(option.MetadataFetcherOrder),
			ImageFetchers:        apiutil.Deref(option.ImageFetchers),
			ImageFetcherOrder:    apiutil.Deref(option.ImageFetcherOrder),
		})
	}

	return options
}
