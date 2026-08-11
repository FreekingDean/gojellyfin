package displaypreferences

import (
	"github.com/FreekingDean/gojellyfin/internal/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func DisplayPreferencesDto(id string, prefs *displaypreferences.DisplayPreferences) api.DisplayPreferencesDto {
	custom := make(map[string]*string, len(prefs.CustomPrefs))
	for key, value := range prefs.CustomPrefs {
		custom[key] = apiutil.Ptr(value)
	}

	return api.DisplayPreferencesDto{
		Id:                 apiutil.Ptr(id),
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

func customPrefs(prefs map[string]*string) map[string]string {
	stored := make(map[string]string, len(prefs))
	for key, value := range prefs {
		stored[key] = apiutil.Deref(value)
	}

	return stored
}
