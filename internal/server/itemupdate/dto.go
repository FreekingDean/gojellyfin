package itemupdate

import (
	"github.com/FreekingDean/gojellyfin/internal/consts"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func metadata(req *api.BaseItemDto) items.Metadata {
	converted := items.Metadata{
		Name:              req.Name,
		SortName:          req.SortName,
		Overview:          req.Overview,
		OfficialRating:    rating(req.OfficialRating),
		CommunityRating:   score(req.CommunityRating),
		ProductionYear:    req.ProductionYear,
		PremiereDate:      req.PremiereDate,
		EndDate:           req.EndDate,
		IndexNumber:       req.IndexNumber,
		ParentIndexNumber: req.ParentIndexNumber,
		Status:            req.Status,
		LockData:          req.LockData,
		Tags:              req.Tags,
		Taglines:          req.Taglines,
	}

	if req.LockedFields != nil {
		fields := make([]string, 0, len(*req.LockedFields))
		for _, field := range *req.LockedFields {
			fields = append(fields, string(field))
		}
		converted.LockedFields = &fields
	}
	if req.ProviderIds != nil {
		providers := make(map[string]string, len(*req.ProviderIds))
		for name, value := range *req.ProviderIds {
			if value != nil {
				providers[name] = *value
			}
		}
		converted.ProviderIds = &providers
	}

	return converted
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

func contentType(collectionType libraries.CollectionType) *api.CollectionType {
	if collectionType == libraries.CollectionTypeMixed {
		return nil
	}

	return apiutil.Ptr(api.CollectionType(collectionType))
}

var contentTypeNames = map[libraries.CollectionType]string{
	libraries.CollectionTypeMovies:      "Movies",
	libraries.CollectionTypeTvshows:     "Shows",
	libraries.CollectionTypeMusic:       "Music",
	libraries.CollectionTypeMusicvideos: "Music Videos",
	libraries.CollectionTypeHomevideos:  "Home Videos",
	libraries.CollectionTypeBoxsets:     "Box Sets",
	libraries.CollectionTypeBooks:       "Books",
	libraries.CollectionTypeMixed:       "Mixed",
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
