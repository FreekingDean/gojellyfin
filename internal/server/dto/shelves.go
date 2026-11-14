package dto

import (
	"context"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
)

func ShelfView(ctx context.Context, collages Collages, shelf shelves.Shelf) (api.BaseItemDto, error) {
	collectionType := api.CollectionType(shelf.Collection)

	tag, err := collages.Tag(ctx, shelf.ID)
	if err != nil {
		return api.BaseItemDto{}, err
	}

	imageTags := map[string]*string{}
	if tag != "" {
		imageTags[string(api.Primary)] = &tag
	}

	return api.BaseItemDto{
		Id:                   apiutil.Ptr(shelf.ID),
		ServerId:             apiutil.Ptr(config.ServerID),
		Name:                 apiutil.Ptr(shelf.Name),
		SortName:             apiutil.Ptr(strings.ToLower(shelf.Name)),
		Type:                 apiutil.Ptr(api.BaseItemKindCollectionFolder),
		CollectionType:       &collectionType,
		DisplayPreferencesId: displayPreferencesID(shelf.ID),
		IsFolder:             apiutil.Ptr(true),
		LocationType:         apiutil.Ptr(api.Virtual),
		ImageTags:            &imageTags,
		BackdropImageTags:    &[]string{},
	}, nil
}
