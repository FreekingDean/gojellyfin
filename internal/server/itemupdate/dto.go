package itemupdate

import (
	"slices"

	"github.com/FreekingDean/gojellyfin/internal/consts"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func metadata(req *api.BaseItemDto) items.Metadata {
	converted := items.Metadata{
		Name:            req.Name,
		SortName:        req.SortName,
		Overview:        req.Overview,
		OfficialRating:  rating(req.OfficialRating),
		CommunityRating: score(req.CommunityRating),
		ProductionYear:  req.ProductionYear,
		PremiereDate:    req.PremiereDate,
		EndDate:         req.EndDate,
		Status:          req.Status,
		LockData:        req.LockData,
		Tags:            req.Tags,
		Taglines:        req.Taglines,
	}

	if req.LockedFields != nil {
		fields := make([]string, 0, len(*req.LockedFields))
		for _, field := range *req.LockedFields {
			fields = append(fields, string(field))
		}
		converted.LockedFields = &fields
	}
	if req.Genres != nil {
		converted.Genres = apiutil.Ptr(slices.Clone(*req.Genres))
	}
	if req.Studios != nil {
		studios := make([]string, 0, len(*req.Studios))
		for _, studio := range *req.Studios {
			if name := apiutil.Deref(studio.Name); name != "" {
				studios = append(studios, name)
			}
		}
		converted.Studios = &studios
	}
	if req.People != nil {
		converted.People = apiutil.Ptr(credits(*req.People))
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

func credits(people []api.BaseItemPerson) []items.Credit {
	converted := make([]items.Credit, 0, len(people))
	for _, person := range people {
		name := apiutil.Deref(person.Name)
		if name == "" {
			continue
		}
		kind := items.CreditKind(apiutil.Deref(person.Type))
		if items.ValidCreditKind(kind) != nil {
			kind = items.CreditKindUnknown
		}
		converted = append(converted, items.Credit{
			Name:  name,
			Kind:  kind,
			Role:  apiutil.Deref(person.Role),
			Order: int32(len(converted)),
		})
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

func contentType(collectionType libraries.CollectionType) *api.CollectionType {
	if collectionType == "" || collectionType == libraries.CollectionTypeMixed {
		return nil
	}

	return apiutil.Ptr(api.CollectionType(collectionType))
}
