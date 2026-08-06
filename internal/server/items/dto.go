package items

import (
	"context"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/google/uuid"
)

var FolderTypes = map[string]bool{
	"Series":           true,
	"Season":           true,
	"Folder":           true,
	"CollectionFolder": true,
}

func ItemDto(item *items.Item, childCount int32, imageTags map[string]string) api.BaseItemDto {
	kind := api.BaseItemKind(item.Type)
	isFolder := FolderTypes[item.Type]

	dto := api.BaseItemDto{
		Id:                &item.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(item.Name),
		SortName:          apiutil.Ptr(item.SortName),
		Type:              &kind,
		Path:              apiutil.Ptr(item.Path),
		IsFolder:          apiutil.Ptr(isFolder),
		ParentId:          item.ParentID,
		IndexNumber:       item.IndexNumber,
		ParentIndexNumber: item.ParentIndexNumber,
		ProductionYear:    item.ProductionYear,
		PremiereDate:      item.PremiereDate,
		RunTimeTicks:      item.RunTimeTicks,
		DateCreated:       apiutil.Ptr(item.CreatedAt),
		LocationType:      apiutil.Ptr(api.FileSystem),
		ImageTags:         &map[string]string{},
		BackdropImageTags: &[]string{},
	}

	if len(imageTags) > 0 {
		tags := map[string]string{}
		backdrops := []string{}
		for imageType, tag := range imageTags {
			if imageType == "Backdrop" {
				backdrops = append(backdrops, tag)
				continue
			}
			tags[imageType] = tag
		}
		dto.ImageTags = &tags
		dto.BackdropImageTags = &backdrops
	}

	if item.Overview != "" {
		dto.Overview = apiutil.Ptr(item.Overview)
	}
	if isFolder {
		dto.ChildCount = apiutil.Ptr(childCount)
	} else {
		dto.MediaType = apiutil.Ptr(api.MediaTypeVideo)
	}

	return dto
}

func ItemDtos(ctx context.Context, store *items.Service, records []items.Item) ([]api.BaseItemDto, error) {
	folderIDs := make([]uuid.UUID, 0, len(records))
	itemIDs := make([]uuid.UUID, 0, len(records))
	for _, item := range records {
		itemIDs = append(itemIDs, item.ID)
		if FolderTypes[item.Type] {
			folderIDs = append(folderIDs, item.ID)
		}
	}

	counts, err := store.CountChildren(ctx, folderIDs)
	if err != nil {
		return nil, err
	}

	imageTags, err := store.ImageTags(ctx, itemIDs)
	if err != nil {
		return nil, err
	}

	userData := map[uuid.UUID]items.Datum{}
	if userID := auth.UserID(ctx); userID != uuid.Nil {
		if userData, err = store.ListUserItemData(ctx, userID, itemIDs); err != nil {
			return nil, err
		}
	}

	converted := make([]api.BaseItemDto, 0, len(records))
	for _, item := range records {
		dto := ItemDto(&item, counts[item.ID], imageTags[item.ID])
		datum, ok := userData[item.ID]
		if !ok {
			datum = items.Datum{ItemID: item.ID}
		}
		dto.UserData = apiutil.Ptr(UserItemDataDto(&datum))
		converted = append(converted, dto)
	}

	return converted, nil
}

func LibraryView(library *libraries.Library) api.BaseItemDto {
	collectionType := api.CollectionType(library.CollectionType)

	return api.BaseItemDto{
		Id:                &library.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(library.Name),
		SortName:          apiutil.Ptr(strings.ToLower(library.Name)),
		Type:              apiutil.Ptr(api.BaseItemKindCollectionFolder),
		CollectionType:    &collectionType,
		IsFolder:          apiutil.Ptr(true),
		LocationType:      apiutil.Ptr(api.FileSystem),
		ImageTags:         &map[string]string{},
		BackdropImageTags: &[]string{},
	}
}

func Descending(order *[]api.SortOrder) bool {
	if order == nil {
		return false
	}

	for _, value := range *order {
		if value == api.Descending {
			return true
		}
	}

	return false
}

func SortFields(sortBy *[]api.ItemSortBy) []string {
	if sortBy == nil {
		return nil
	}

	fields := make([]string, 0, len(*sortBy))
	for _, field := range *sortBy {
		fields = append(fields, string(field))
	}

	return fields
}

func UserItemDataDto(datum *items.Datum) api.UserItemDataDto {
	return api.UserItemDataDto{
		ItemId:                &datum.ItemID,
		Key:                   apiutil.Ptr(datum.ItemID.String()),
		Played:                apiutil.Ptr(datum.Played),
		PlayCount:             apiutil.Ptr(datum.PlayCount),
		IsFavorite:            apiutil.Ptr(datum.IsFavorite),
		PlaybackPositionTicks: apiutil.Ptr(datum.PlaybackPositionTicks),
		LastPlayedDate:        datum.LastPlayedDate,
	}
}
