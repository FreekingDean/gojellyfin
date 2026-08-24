package librarystructure

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func requestedPaths(params *[]string, options *api.LibraryOptions) []string {
	paths := make([]string, 0)
	add := func(path string) {
		if path != "" && !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}

	for _, path := range apiutil.Deref(params) {
		add(path)
	}
	if options != nil && options.PathInfos != nil {
		for _, info := range *options.PathInfos {
			add(apiutil.Deref(info.Path))
		}
	}

	return paths
}

func virtualFolderInfo(library *libraries.Library) api.VirtualFolderInfo {
	options := optionsDto(library.Edges.Options)

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

	update := s.libraries.UpdateOptions(id).
		SetNillableEnabled(req.Enabled).
		SetNillableEnablePhotos(req.EnablePhotos).
		SetNillableEnableRealtimeMonitor(req.EnableRealtimeMonitor).
		SetNillableEnableLufsScan(req.EnableLUFSScan).
		SetNillableEnableChapterImageExtraction(req.EnableChapterImageExtraction).
		SetNillableExtractChapterImagesDuringLibraryScan(req.ExtractChapterImagesDuringLibraryScan).
		SetNillableEnableTrickplayImageExtraction(req.EnableTrickplayImageExtraction).
		SetNillableExtractTrickplayImagesDuringLibraryScan(req.ExtractTrickplayImagesDuringLibraryScan).
		SetNillableSaveLocalMetadata(req.SaveLocalMetadata).
		// Deprecated upstream with no replacement, and clients still send it.
		SetNillableEnableInternetProviders(req.EnableInternetProviders). //nolint:staticcheck
		SetNillableEnableAutomaticSeriesGrouping(req.EnableAutomaticSeriesGrouping).
		SetNillableEnableEmbeddedTitles(req.EnableEmbeddedTitles).
		SetNillableEnableEmbeddedExtrasTitles(req.EnableEmbeddedExtrasTitles).
		SetNillableEnableEmbeddedEpisodeInfos(req.EnableEmbeddedEpisodeInfos).
		SetNillableSkipSubtitlesIfEmbeddedSubtitlesPresent(req.SkipSubtitlesIfEmbeddedSubtitlesPresent).
		SetNillableSkipSubtitlesIfAudioTrackMatches(req.SkipSubtitlesIfAudioTrackMatches).
		SetNillableRequirePerfectSubtitleMatch(req.RequirePerfectSubtitleMatch).
		SetNillableSaveSubtitlesWithMedia(req.SaveSubtitlesWithMedia).
		SetNillableSaveLyricsWithMedia(req.SaveLyricsWithMedia).
		SetNillableSaveTrickplayWithMedia(req.SaveTrickplayWithMedia).
		SetNillablePreferNonstandardArtistsTag(req.PreferNonstandardArtistsTag).
		SetNillableUseCustomTagDelimiters(req.UseCustomTagDelimiters).
		SetNillableAutomaticallyAddToCollection(req.AutomaticallyAddToCollection).
		SetNillableAutomaticRefreshIntervalDays(req.AutomaticRefreshIntervalDays).
		SetNillablePreferredMetadataLanguage(req.PreferredMetadataLanguage).
		SetNillableMetadataCountryCode(req.MetadataCountryCode).
		SetNillableSeasonZeroDisplayName(req.SeasonZeroDisplayName)

	if req.AllowEmbeddedSubtitles != nil {
		update.SetAllowEmbeddedSubtitles(libraries.EmbeddedSubtitles(*req.AllowEmbeddedSubtitles))
	}
	if req.MetadataSavers != nil {
		update.SetMetadataSavers(*req.MetadataSavers)
	}
	if req.DisabledLocalMetadataReaders != nil {
		update.SetDisabledLocalMetadataReaders(*req.DisabledLocalMetadataReaders)
	}
	if req.LocalMetadataReaderOrder != nil {
		update.SetLocalMetadataReaderOrder(*req.LocalMetadataReaderOrder)
	}
	if req.DisabledSubtitleFetchers != nil {
		update.SetDisabledSubtitleFetchers(*req.DisabledSubtitleFetchers)
	}
	if req.SubtitleFetcherOrder != nil {
		update.SetSubtitleFetcherOrder(*req.SubtitleFetcherOrder)
	}
	if req.DisabledMediaSegmentProviders != nil {
		update.SetDisabledMediaSegmentProviders(*req.DisabledMediaSegmentProviders)
	}
	if req.SubtitleDownloadLanguages != nil {
		update.SetSubtitleDownloadLanguages(*req.SubtitleDownloadLanguages)
	}
	if req.DisabledLyricFetchers != nil {
		update.SetDisabledLyricFetchers(*req.DisabledLyricFetchers)
	}
	if req.LyricFetcherOrder != nil {
		update.SetLyricFetcherOrder(*req.LyricFetcherOrder)
	}
	if req.CustomTagDelimiters != nil {
		update.SetCustomTagDelimiters(*req.CustomTagDelimiters)
	}
	if req.DelimiterWhitelist != nil {
		update.SetDelimiterWhitelist(*req.DelimiterWhitelist)
	}
	if req.TypeOptions != nil {
		typeOptions := make([]libraries.TypeOptions, 0, len(*req.TypeOptions))
		for _, option := range *req.TypeOptions {
			typeOptions = append(typeOptions, libraries.TypeOptions{
				Type:                 apiutil.Deref(option.Type),
				MetadataFetchers:     apiutil.Deref(option.MetadataFetchers),
				MetadataFetcherOrder: apiutil.Deref(option.MetadataFetcherOrder),
				ImageFetchers:        apiutil.Deref(option.ImageFetchers),
				ImageFetcherOrder:    apiutil.Deref(option.ImageFetcherOrder),
			})
		}
		update.SetTypeOptions(typeOptions)
	}

	return update.Exec(ctx)
}
