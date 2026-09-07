package dto

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

var musicKinds = map[items.Kind]bool{
	itemmodal.KindAudio:      true,
	itemmodal.KindMusicAlbum: true,
}

func applyMusicFields(ctx context.Context, store *items.Service, records []*items.Item, converted []api.BaseItemDto) error {
	parentIDs := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		if record.ParentID != nil && musicKinds[record.Kind] {
			parentIDs = append(parentIDs, *record.ParentID)
		}
	}
	if len(parentIDs) == 0 {
		return nil
	}

	parents, err := store.ItemsByIDs(ctx, parentIDs)
	if err != nil {
		return err
	}

	artistIDs := make([]uuid.UUID, 0, len(parents))
	for _, parent := range parents {
		if parent.Kind == itemmodal.KindMusicAlbum && parent.ParentID != nil {
			artistIDs = append(artistIDs, *parent.ParentID)
		}
	}

	artists, err := store.ItemsByIDs(ctx, artistIDs)
	if err != nil {
		return err
	}

	for index, record := range records {
		if record.ParentID == nil || !musicKinds[record.Kind] {
			continue
		}

		parent := parents[*record.ParentID]
		if parent == nil {
			continue
		}

		artist := parent
		if parent.Kind == itemmodal.KindMusicAlbum {
			converted[index].Album = apiutil.Ptr(parent.Name)
			converted[index].AlbumId = apiutil.Ptr(parent.ID)
			if parent.ParentID == nil {
				continue
			}
			artist = artists[*parent.ParentID]
		}
		if artist == nil || artist.Kind != itemmodal.KindMusicArtist {
			continue
		}

		pair := []api.NameGuidPair{{Id: apiutil.Ptr(artist.ID), Name: apiutil.Ptr(artist.Name)}}
		converted[index].AlbumArtist = apiutil.Ptr(artist.Name)
		converted[index].AlbumArtists = &pair
		converted[index].ArtistItems = &pair
		converted[index].Artists = &[]string{artist.Name}
	}

	return nil
}

func GenreDto(genre items.Named, kind api.BaseItemKind) api.BaseItemDto {
	return api.BaseItemDto{
		Id:                &genre.ID,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(genre.Name),
		SortName:          apiutil.Ptr(genre.Name),
		Type:              &kind,
		IsFolder:          apiutil.Ptr(true),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}
