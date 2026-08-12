package playlists

import (
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func playlistDto(playlist *playlists.Playlist, entries []*playlists.Entry, shares []*playlists.Share) api.PlaylistDto {
	itemIDs := make([]uuid.UUID, 0, len(entries))
	for _, entry := range entries {
		itemIDs = append(itemIDs, entry.ItemID)
	}

	return api.PlaylistDto{
		ItemIds:    itemIDs,
		OpenAccess: apiutil.Ptr(playlist.OpenAccess),
		Shares:     userPermissions(shares),
	}
}

func userPermissions(shares []*playlists.Share) []api.PlaylistUserPermissions {
	converted := make([]api.PlaylistUserPermissions, 0, len(shares))
	for _, share := range shares {
		converted = append(converted, userPermission(share))
	}

	return converted
}

func userPermission(share *playlists.Share) api.PlaylistUserPermissions {
	return api.PlaylistUserPermissions{
		UserId:  &share.UserID,
		CanEdit: apiutil.Ptr(share.CanEdit),
	}
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
