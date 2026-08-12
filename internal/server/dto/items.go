package dto

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

func ItemDto(item *items.Item, childCount int32, imageTags map[string]string) api.BaseItemDto {
	kind := api.BaseItemKind(item.Kind)

	dto := api.BaseItemDto{
		Id:                &item.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(item.Name),
		SortName:          apiutil.Ptr(item.SortName),
		Type:              &kind,
		Path:              apiutil.Ptr(item.Path),
		IsFolder:          apiutil.Ptr(item.IsFolder),
		ParentId:          item.ParentID,
		IndexNumber:       item.IndexNumber,
		ParentIndexNumber: item.ParentIndexNumber,
		ProductionYear:    item.ProductionYear,
		PremiereDate:      item.PremiereDate,
		RunTimeTicks:      item.RunTimeTicks,
		DateCreated:       apiutil.Ptr(item.CreatedAt),
		LocationType:      apiutil.Ptr(api.LocationType(item.LocationType)),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}

	if len(imageTags) > 0 {
		tags := map[string]*string{}
		backdrops := []string{}
		for imageType, tag := range imageTags {
			if imageType == "Backdrop" {
				backdrops = append(backdrops, tag)
				continue
			}
			tags[imageType] = &tag
		}
		dto.ImageTags = &tags
		dto.BackdropImageTags = &backdrops
	}

	if item.Overview != "" {
		dto.Overview = apiutil.Ptr(item.Overview)
	}
	if item.IsFolder {
		dto.ChildCount = apiutil.Ptr(childCount)
	} else {
		dto.MediaType = apiutil.Ptr(api.MediaType(item.MediaType))
		dto.HasSubtitles = apiutil.Ptr(item.HasSubtitles)
	}

	return dto
}

func ItemDtos(ctx context.Context, store *items.Service, records []*items.Item) ([]api.BaseItemDto, error) {
	folderIDs := make([]uuid.UUID, 0, len(records))
	itemIDs := make([]uuid.UUID, 0, len(records))
	for _, item := range records {
		itemIDs = append(itemIDs, item.ID)
		if item.IsFolder {
			folderIDs = append(folderIDs, item.ID)
		}
	}

	counts, err := store.CountChildren(ctx, folderIDs)
	if err != nil {
		return nil, err
	}

	imageTags, err := store.ImageTagsByItem(ctx, itemIDs)
	if err != nil {
		return nil, err
	}

	userData := map[uuid.UUID]*items.Datum{}
	if userID := auth.UserID(ctx); userID != uuid.Nil {
		if userData, err = store.ListUserItemData(ctx, userID, itemIDs); err != nil {
			return nil, err
		}
	}

	converted := make([]api.BaseItemDto, 0, len(records))
	for _, item := range records {
		dto := ItemDto(item, counts[item.ID], imageTags[item.ID])
		datum, ok := userData[item.ID]
		if !ok {
			datum = &items.Datum{ItemID: item.ID}
		}
		dto.UserData = apiutil.Ptr(UserItemDataDto(datum))
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
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}

func RootView() api.BaseItemDto {
	return api.BaseItemDto{
		Id:       apiutil.UID(config.RootFolderID),
		Name:     apiutil.Ptr("Media Folders"),
		ServerId: apiutil.Ptr(config.ServerID),
		Type:     apiutil.Ptr(api.BaseItemKindFolder),
		IsFolder: apiutil.Ptr(true),
	}
}

func UserItemDataDto(datum *items.Datum) api.UserItemDataDto {
	return api.UserItemDataDto{
		ItemId:                &datum.ItemID,
		Key:                   datum.ItemID.String(),
		Played:                apiutil.Ptr(datum.Played),
		PlayCount:             apiutil.Ptr(datum.PlayCount),
		IsFavorite:            apiutil.Ptr(datum.IsFavorite),
		PlaybackPositionTicks: apiutil.Ptr(datum.PlaybackPositionTicks),
		LastPlayedDate:        datum.LastPlayedAt,
	}
}

func MediaTypes(types *[]api.MediaType) []items.MediaType {
	if types == nil {
		return nil
	}

	mediaTypes := make([]items.MediaType, 0, len(*types))
	for _, value := range *types {
		mediaType := items.MediaType(value)
		if items.ValidMediaType(mediaType) == nil {
			mediaTypes = append(mediaTypes, mediaType)
		}
	}

	return mediaTypes
}

func Kinds(types *[]api.BaseItemKind) []items.Kind {
	if types == nil {
		return nil
	}

	kinds := make([]items.Kind, 0, len(*types))
	for _, value := range *types {
		kind := items.Kind(value)
		if items.ValidKind(kind) == nil {
			kinds = append(kinds, kind)
		}
	}

	return kinds
}
