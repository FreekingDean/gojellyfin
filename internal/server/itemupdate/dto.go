package itemupdate

import (
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

// Everything the editor may change; the columns the scanner and the probe own
// have no home here on purpose.
func Metadata(req *api.BaseItemDto) items.Metadata {
	metadata := items.Metadata{
		Name:                         req.Name,
		OriginalTitle:                req.OriginalTitle,
		SortName:                     req.SortName,
		Overview:                     req.Overview,
		OfficialRating:               req.OfficialRating,
		CustomRating:                 req.CustomRating,
		CommunityRating:              rating(req.CommunityRating),
		CriticRating:                 rating(req.CriticRating),
		ProductionYear:               req.ProductionYear,
		PremiereDate:                 req.PremiereDate,
		EndDate:                      req.EndDate,
		IndexNumber:                  req.IndexNumber,
		IndexNumberEnd:               req.IndexNumberEnd,
		ParentIndexNumber:            req.ParentIndexNumber,
		AirsBeforeSeasonNumber:       req.AirsBeforeSeasonNumber,
		AirsAfterSeasonNumber:        req.AirsAfterSeasonNumber,
		AirsBeforeEpisodeNumber:      req.AirsBeforeEpisodeNumber,
		Status:                       req.Status,
		AirTime:                      req.AirTime,
		DisplayOrder:                 req.DisplayOrder,
		LockData:                     req.LockData,
		PreferredMetadataLanguage:    req.PreferredMetadataLanguage,
		PreferredMetadataCountryCode: req.PreferredMetadataCountryCode,
		Tags:                         req.Tags,
		Taglines:                     req.Taglines,
		ProductionLocations:          req.ProductionLocations,
	}

	if req.AirDays != nil {
		days := make([]string, 0, len(*req.AirDays))
		for _, day := range *req.AirDays {
			days = append(days, string(day))
		}
		metadata.AirDays = &days
	}
	if req.LockedFields != nil {
		fields := make([]string, 0, len(*req.LockedFields))
		for _, field := range *req.LockedFields {
			fields = append(fields, string(field))
		}
		metadata.LockedFields = &fields
	}
	if req.ProviderIds != nil {
		providers := make(map[string]string, len(*req.ProviderIds))
		for name, value := range *req.ProviderIds {
			if value != nil {
				providers[name] = *value
			}
		}
		metadata.ProviderIds = &providers
	}

	return metadata
}

func rating(value *float32) *float64 {
	if value == nil {
		return nil
	}

	return apiutil.Ptr(float64(*value))
}

// Content types live in the server configuration keyed by path, the same place
// Jellyfin keeps them.
func configuredContentType(server api.ServerConfiguration, path string) string {
	for _, pair := range apiutil.Deref(server.ContentTypes) {
		if strings.EqualFold(apiutil.Deref(pair.Name), path) {
			return apiutil.Deref(pair.Value)
		}
	}

	return ""
}

func contentTypeOptions() []api.NameValuePair {
	return []api.NameValuePair{
		contentTypeOption("Inherit", ""),
		contentTypeOption("Movies", api.CollectionTypeMovies),
		contentTypeOption("Music", api.CollectionTypeMusic),
		contentTypeOption("Shows", api.CollectionTypeTvshows),
		contentTypeOption("Books", api.CollectionTypeBooks),
		contentTypeOption("Home Videos", api.CollectionTypeHomevideos),
		contentTypeOption("Music Videos", api.CollectionTypeMusicvideos),
		contentTypeOption("Photos", api.CollectionTypePhotos),
	}
}

func contentTypeOption(name string, value api.CollectionType) api.NameValuePair {
	return api.NameValuePair{Name: apiutil.Ptr(name), Value: apiutil.Ptr(string(value))}
}
