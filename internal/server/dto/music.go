package dto

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func applyMusicFields(ctx context.Context, store *items.Service, records []*items.Item, converted []api.BaseItemDto) error {
	parentIDs := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		if record.ParentID != nil && (record.Kind == itemmodel.KindAudio || record.Kind == itemmodel.KindMusicAlbum) {
			parentIDs = append(parentIDs, *record.ParentID)
		}
	}
	if len(parentIDs) == 0 {
		return nil
	}

	parents, err := store.ItemsByIDs(ctx, items.Everyone, parentIDs)
	if err != nil {
		return err
	}

	artistIDs := make([]uuid.UUID, 0, len(parents))
	albumIDs := make([]uuid.UUID, 0, len(parents))
	for _, parent := range parents {
		if parent.Kind == itemmodel.KindMusicAlbum {
			albumIDs = append(albumIDs, parent.ID)
			if parent.ParentID != nil {
				artistIDs = append(artistIDs, *parent.ParentID)
			}
		}
	}

	artists, err := store.ItemsByIDs(ctx, items.Everyone, artistIDs)
	if err != nil {
		return err
	}

	tags, err := store.ImageTagsByItem(ctx, albumIDs)
	if err != nil {
		return err
	}

	for index, record := range records {
		if record.ParentID == nil {
			continue
		}

		parent := parents[*record.ParentID]
		if parent == nil {
			continue
		}

		artist := parent
		if record.Kind == itemmodel.KindAudio {
			if parent.Kind != itemmodel.KindMusicAlbum {
				continue
			}

			converted[index].Album = apiutil.Ptr(parent.Name)
			converted[index].AlbumId = apiutil.Ptr(parent.ID)
			if tag, ok := tags[parent.ID]["Primary"]; ok {
				converted[index].AlbumPrimaryImageTag = apiutil.Ptr(tag)
			}
			if parent.ParentID == nil {
				continue
			}
			artist = artists[*parent.ParentID]
		}
		if artist == nil || artist.Kind != itemmodel.KindMusicArtist {
			continue
		}

		credited := []api.NameGuidPair{{Id: apiutil.Ptr(artist.ID), Name: apiutil.Ptr(artist.Name)}}
		converted[index].AlbumArtist = apiutil.Ptr(artist.Name)
		converted[index].AlbumArtists = apiutil.Ptr(credited)
		converted[index].ArtistItems = apiutil.Ptr(credited)
		converted[index].Artists = apiutil.Ptr([]string{artist.Name})
	}

	return nil
}
