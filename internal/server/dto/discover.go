package dto

import (
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func UnownedDtos(titles []discover.Title) []api.BaseItemDto {
	converted := make([]api.BaseItemDto, 0, len(titles))
	for _, title := range titles {
		converted = append(converted, UnownedDto(title))
	}

	return converted
}

func UnownedDto(title discover.Title) api.BaseItemDto {
	kind := api.BaseItemKind(title.Kind)
	metadata := title.Metadata
	name := apiutil.Deref(metadata.Name)

	imageTags := map[string]*string{}
	if url, found := title.Image(items.ImageKindPrimary); found {
		imageTags[string(api.Primary)] = apiutil.Ptr(discover.Tag(url))
	}
	backdrops := []string{}
	if url, found := title.Image(items.ImageKindBackdrop); found {
		backdrops = append(backdrops, discover.Tag(url))
	}

	dto := api.BaseItemDto{
		Id:                apiutil.Ptr(title.ID),
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(name),
		SortName:          apiutil.Ptr(items.SortName(name)),
		Type:              &kind,
		IsFolder:          apiutil.Ptr(items.IsFolder(title.Kind)),
		LocationType:      apiutil.Ptr(api.Virtual),
		PlayAccess:        apiutil.Ptr(api.PlayAccessNone),
		CanDelete:         apiutil.Ptr(false),
		CanDownload:       apiutil.Ptr(false),
		Overview:          metadata.Overview,
		ProductionYear:    metadata.ProductionYear,
		PremiereDate:      metadata.PremiereDate,
		EndDate:           metadata.EndDate,
		RunTimeTicks:      metadata.RunTimeTicks,
		Status:            metadata.Status,
		Taglines:          metadata.Taglines,
		Genres:            metadata.Genres,
		ImageTags:         &imageTags,
		BackdropImageTags: &backdrops,
		UserData:          apiutil.Ptr(UserItemDataDto(&items.Datum{ItemID: title.ID})),
	}

	if metadata.OfficialRating != nil {
		dto.OfficialRating = apiutil.Ptr(string(*metadata.OfficialRating))
	}
	if metadata.CommunityRating != nil {
		dto.CommunityRating = apiutil.Ptr(float32(*metadata.CommunityRating))
	}
	if items.IsFolder(title.Kind) {
		dto.ChildCount = apiutil.Ptr(int32(0))
	} else {
		dto.MediaType = apiutil.Ptr(api.MediaType(items.MediaTypeOf(title.Kind)))
	}

	return dto
}
