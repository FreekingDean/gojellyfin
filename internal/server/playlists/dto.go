package playlists

import (
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func PlaylistDto(playlist *playlists.Playlist, entries []*playlists.Entry, shares []*playlists.Share) api.PlaylistDto {
	itemIDs := make([]uuid.UUID, 0, len(entries))
	for _, entry := range entries {
		if entry.Edges.Item != nil {
			itemIDs = append(itemIDs, entry.Edges.Item.ID)
		}
	}

	return api.PlaylistDto{
		ItemIds:    &itemIDs,
		OpenAccess: apiutil.Ptr(playlist.OpenAccess),
		Shares:     apiutil.Ptr(UserPermissions(shares)),
	}
}

func UserPermissions(shares []*playlists.Share) []api.PlaylistUserPermissions {
	converted := make([]api.PlaylistUserPermissions, 0, len(shares))
	for _, share := range shares {
		converted = append(converted, UserPermission(share))
	}

	return converted
}

func UserPermission(share *playlists.Share) api.PlaylistUserPermissions {
	permission := api.PlaylistUserPermissions{CanEdit: apiutil.Ptr(share.CanEdit)}
	if share.Edges.User != nil {
		permission.UserId = &share.Edges.User.ID
	}

	return permission
}

func permissions(users *[]api.PlaylistUserPermissions) []playlists.Permission {
	if users == nil {
		return nil
	}

	converted := make([]playlists.Permission, 0, len(*users))
	for _, user := range *users {
		converted = append(converted, playlists.Permission{
			UserID:  apiutil.Deref(user.UserId),
			CanEdit: apiutil.Deref(user.CanEdit),
		})
	}

	return converted
}

func mediaType(value *api.MediaType) playlists.MediaType {
	kind := playlists.MediaType(apiutil.Deref(value))
	if playlists.ValidMediaType(kind) != nil {
		return playlists.MediaTypeUnknown
	}

	return kind
}
