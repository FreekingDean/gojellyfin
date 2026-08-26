package displaypreferences

import (
	"github.com/FreekingDean/gojellyfin/internal/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func displayPreferencesDto(prefs *displaypreferences.DisplayPreferences) api.DisplayPreferencesDto {
	custom := make(map[string]*string, len(prefs.CustomPrefs))
	for key, value := range prefs.CustomPrefs {
		custom[key] = apiutil.Ptr(value)
	}

	return api.DisplayPreferencesDto{
		Id:                 apiutil.Ptr(prefs.ReferenceID),
		Client:             apiutil.Ptr(prefs.Client),
		ViewType:           apiutil.Ptr(prefs.ViewType),
		SortBy:             apiutil.Ptr(prefs.SortBy),
		IndexBy:            apiutil.Ptr(prefs.IndexBy),
		SortOrder:          apiutil.Ptr(api.SortOrder(prefs.SortOrder)),
		ScrollDirection:    apiutil.Ptr(api.ScrollDirection(prefs.ScrollDirection)),
		RememberIndexing:   apiutil.Ptr(prefs.RememberIndexing),
		RememberSorting:    apiutil.Ptr(prefs.RememberSorting),
		ShowBackdrop:       apiutil.Ptr(prefs.ShowBackdrop),
		ShowSidebar:        apiutil.Ptr(prefs.ShowSidebar),
		PrimaryImageHeight: apiutil.Ptr(prefs.PrimaryImageHeight),
		PrimaryImageWidth:  apiutil.Ptr(prefs.PrimaryImageWidth),
		CustomPrefs:        &custom,
	}
}

func settings(req *api.DisplayPreferencesDto) displaypreferences.Settings {
	converted := displaypreferences.Settings{
		ViewType:           req.ViewType,
		SortBy:             req.SortBy,
		IndexBy:            req.IndexBy,
		RememberIndexing:   req.RememberIndexing,
		RememberSorting:    req.RememberSorting,
		ShowBackdrop:       req.ShowBackdrop,
		ShowSidebar:        req.ShowSidebar,
		PrimaryImageHeight: req.PrimaryImageHeight,
		PrimaryImageWidth:  req.PrimaryImageWidth,
	}

	if req.SortOrder != nil {
		converted.SortOrder = apiutil.Ptr(displaypreferences.SortOrder(*req.SortOrder))
	}
	if req.ScrollDirection != nil {
		converted.ScrollDirection = apiutil.Ptr(displaypreferences.ScrollDirection(*req.ScrollDirection))
	}
	if req.CustomPrefs != nil {
		converted.CustomPrefs = apiutil.Ptr(customPrefs(*req.CustomPrefs))
	}

	return converted
}

func customPrefs(prefs map[string]*string) map[string]string {
	stored := make(map[string]string, len(prefs))
	for key, value := range prefs {
		if value == nil {
			continue
		}
		stored[key] = *value
	}

	return stored
}
