package dto

import (
	"context"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/google/uuid"
)

func runtime(item *items.Item, held items.Held) *int64 {
	if item.RunTimeTicks != nil {
		return item.RunTimeTicks
	}
	if held.RunTimeTicks == nil || *held.RunTimeTicks == 0 {
		return nil
	}

	return held.RunTimeTicks
}

func ItemDto(item *items.Item, held items.Held, childCount int32, imageTags map[string]string) api.BaseItemDto {
	kind := api.BaseItemKind(item.Kind)

	dto := api.BaseItemDto{
		Id:                &item.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(item.Name),
		SortName:          apiutil.Ptr(item.SortName),
		Type:              &kind,
		Path:              apiutil.Ptr(held.Path),
		IsFolder:          apiutil.Ptr(items.IsFolder(item.Kind)),
		LockData:          apiutil.Ptr(item.LockData),
		ParentId:          item.ParentID,
		IndexNumber:       item.IndexNumber,
		ParentIndexNumber: item.ParentIndexNumber,
		ProductionYear:    item.ProductionYear,
		PremiereDate:      item.PremiereDate,
		RunTimeTicks:      runtime(item, held),
		DateCreated:       apiutil.Ptr(item.CreatedAt),
		LocationType:      apiutil.Ptr(api.FileSystem),
		CanDelete:         apiutil.Ptr(reports.Takes(item.Kind)),
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
		if _, ok := tags["Primary"]; ok {
			dto.PrimaryImageAspectRatio = apiutil.Ptr(primaryAspect(kind))
		}
	}

	if item.Overview != "" {
		dto.Overview = apiutil.Ptr(item.Overview)
	}
	if item.OfficialRating != "" {
		dto.OfficialRating = apiutil.Ptr(item.OfficialRating)
	}
	if item.Status != "" {
		dto.Status = apiutil.Ptr(item.Status)
	}
	if item.CommunityRating != nil {
		dto.CommunityRating = apiutil.Ptr(float32(*item.CommunityRating))
	}
	if len(item.Taglines) > 0 {
		dto.Taglines = apiutil.Ptr(item.Taglines)
	}
	dto.EndDate = item.EndDate
	if len(item.LockedFields) > 0 {
		locked := make([]api.MetadataField, 0, len(item.LockedFields))
		for _, field := range item.LockedFields {
			locked = append(locked, api.MetadataField(field))
		}
		dto.LockedFields = &locked
	}
	if items.IsFolder(item.Kind) {
		dto.ChildCount = apiutil.Ptr(childCount)
	} else {
		dto.MediaType = apiutil.Ptr(api.MediaType(items.MediaTypeOf(item.Kind)))
		dto.HasSubtitles = apiutil.Ptr(held.HasSubtitles)
		dto.HasLyrics = apiutil.Ptr(held.HasLyrics)
	}

	return dto
}

const (
	posterAspect = 2.0 / 3.0
	stillAspect  = 16.0 / 9.0
)

func primaryAspect(kind api.BaseItemKind) float64 {
	if kind == api.BaseItemKindEpisode {
		return stillAspect
	}

	return posterAspect
}

func ItemDtos(ctx context.Context, store *items.Service, records []*items.Item) ([]api.BaseItemDto, error) {
	folderIDs := make([]uuid.UUID, 0, len(records))
	itemIDs := make([]uuid.UUID, 0, len(records))
	for _, item := range records {
		itemIDs = append(itemIDs, item.ID)
		if items.IsFolder(item.Kind) {
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

	held, err := store.FilesByItem(ctx, itemIDs)
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
		dto := ItemDto(item, held[item.ID], counts[item.ID], imageTags[item.ID])
		datum, ok := userData[item.ID]
		if !ok {
			datum = &items.Datum{ItemID: item.ID}
		}
		dto.UserData = apiutil.Ptr(UserItemDataDto(datum))
		converted = append(converted, dto)
	}

	if err := applyShowFields(ctx, store, records, converted); err != nil {
		return nil, err
	}
	if err := applyMusicFields(ctx, store, records, converted); err != nil {
		return nil, err
	}

	return converted, nil
}

type Cards interface {
	Tag(ctx context.Context, id, userID uuid.UUID) (string, error)
}

func LibraryViews(ctx context.Context, cards Cards, records []*libraries.Library) ([]api.BaseItemDto, error) {
	views := make([]api.BaseItemDto, 0, len(records))
	for _, library := range records {
		view, err := LibraryView(ctx, cards, library)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}

	return views, nil
}

func LibraryView(ctx context.Context, cards Cards, library *libraries.Library) (api.BaseItemDto, error) {
	collectionType := api.CollectionType(library.CollectionType)

	tag, err := cards.Tag(ctx, library.ID, uuid.Nil)
	if err != nil {
		return api.BaseItemDto{}, err
	}

	imageTags := map[string]*string{}
	if tag != "" {
		imageTags[string(api.Primary)] = &tag
	}

	return api.BaseItemDto{
		Id:                   &library.ID,
		ServerId:             apiutil.Ptr(config.ServerID),
		Name:                 apiutil.Ptr(library.Name),
		SortName:             apiutil.Ptr(strings.ToLower(library.Name)),
		Type:                 apiutil.Ptr(api.BaseItemKindCollectionFolder),
		CollectionType:       &collectionType,
		DisplayPreferencesId: displayPreferencesID(library.ID),
		IsFolder:             apiutil.Ptr(true),
		LocationType:         apiutil.Ptr(api.FileSystem),
		ImageTags:            &imageTags,
		BackdropImageTags:    &[]string{},
	}, nil
}

func displayPreferencesID(id uuid.UUID) *string {
	return apiutil.Ptr(strings.ReplaceAll(id.String(), "-", ""))
}

func RootView() api.BaseItemDto {
	return api.BaseItemDto{
		Id:       apiutil.Ptr(config.RootFolderUID),
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

func LatestDtos(ctx context.Context, store *items.Service, latest []items.Latest) ([]api.BaseItemDto, error) {
	records := make([]*items.Item, 0, len(latest))
	for _, entry := range latest {
		records = append(records, entry.Item)
	}

	converted, err := ItemDtos(ctx, store, records)
	if err != nil {
		return nil, err
	}

	for index, entry := range latest {
		converted[index].ChildCount = apiutil.Ptr(entry.Count)
	}

	return converted, nil
}
