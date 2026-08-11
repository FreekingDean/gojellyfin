package itemupdate

import (
	"github.com/FreekingDean/gojellyfin/internal/consts"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
)

// Everything the editor may change; the columns the scanner and the probe own
// have no home here on purpose.
func Metadata(req *api.BaseItemDto) items.Metadata {
	metadata := items.Metadata{
		Name:                         req.Name,
		OriginalTitle:                req.OriginalTitle,
		SortName:                     req.SortName,
		Overview:                     req.Overview,
		OfficialRating:               rating(req.OfficialRating),
		CustomRating:                 rating(req.CustomRating),
		CommunityRating:              score(req.CommunityRating),
		CriticRating:                 score(req.CriticRating),
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

func rating(value *string) *consts.Rating {
	return (*consts.Rating)(value)
}

func score(value *float32) *float64 {
	if value == nil {
		return nil
	}

	return apiutil.Ptr(float64(*value))
}

func supportedContentType(value *string) bool {
	contentType := libraries.CollectionType(apiutil.Deref(value))

	return contentType == "" || libraries.ValidCollectionType(contentType) == nil
}

// Mixed is the absence of a content type rather than one the api can name.
func contentType(collectionType libraries.CollectionType) *api.CollectionType {
	if collectionType == librarymodal.CollectionTypeMixed {
		return nil
	}

	return apiutil.Ptr(api.CollectionType(collectionType))
}

var contentTypeNames = map[libraries.CollectionType]string{
	librarymodal.CollectionTypeMovies:      "Movies",
	librarymodal.CollectionTypeTvshows:     "Shows",
	librarymodal.CollectionTypeMusic:       "Music",
	librarymodal.CollectionTypeMusicvideos: "Music Videos",
	librarymodal.CollectionTypeHomevideos:  "Home Videos",
	librarymodal.CollectionTypeBoxsets:     "Box Sets",
	librarymodal.CollectionTypeBooks:       "Books",
	librarymodal.CollectionTypeMixed:       "Mixed",
}

func contentTypeOptions() []api.NameValuePair {
	options := make([]api.NameValuePair, 0, len(libraries.CollectionTypes))
	for _, collectionType := range libraries.CollectionTypes {
		options = append(options, api.NameValuePair{
			Name:  apiutil.Ptr(contentTypeNames[collectionType]),
			Value: apiutil.Ptr(string(collectionType)),
		})
	}

	return options
}
