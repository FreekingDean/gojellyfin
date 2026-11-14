package dto

import (
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
)

func ShelfView(shelf shelves.Shelf) api.BaseItemDto {
	collectionType := api.CollectionType(shelf.Collection)

	return api.BaseItemDto{
		Id:                apiutil.Ptr(shelf.ID),
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(shelf.Name),
		SortName:          apiutil.Ptr(strings.ToLower(shelf.Name)),
		Type:              apiutil.Ptr(api.BaseItemKindCollectionFolder),
		CollectionType:    &collectionType,
		IsFolder:          apiutil.Ptr(true),
		LocationType:      apiutil.Ptr(api.Virtual),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}
