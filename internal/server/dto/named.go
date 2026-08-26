package dto

import (
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func NamedItem(named items.Named, kind api.BaseItemKind, isFolder bool) api.BaseItemDto {
	return api.BaseItemDto{
		Id:                &named.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(named.Name),
		SortName:          apiutil.Ptr(named.Name),
		Type:              &kind,
		IsFolder:          apiutil.Ptr(isFolder),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}
