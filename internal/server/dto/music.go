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

	parents, artists, tags, err := musicLineage(ctx, store, parentIDs)
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

			onAlbum(&converted[index], parent, tags[parent.ID])
			if parent.ParentID == nil {
				continue
			}
			artist = artists[*parent.ParentID]
		}
		if artist == nil || artist.Kind != itemmodel.KindMusicArtist {
			continue
		}

		byArtist(&converted[index], artist)
	}

	return nil
}

func musicLineage(ctx context.Context, store *items.Service, parentIDs []uuid.UUID) (map[uuid.UUID]*items.Item, map[uuid.UUID]*items.Item, map[uuid.UUID]map[string]string, error) {
	parents, err := store.ItemsByIDs(ctx, items.Everyone, parentIDs)
	if err != nil {
		return nil, nil, nil, err
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
		return nil, nil, nil, err
	}

	tags, err := store.ImageTagsByItem(ctx, albumIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	return parents, artists, tags, nil
}

func onAlbum(dto *api.BaseItemDto, album *items.Item, tags map[string]string) {
	dto.Album = apiutil.Ptr(album.Name)
	dto.AlbumId = apiutil.Ptr(album.ID)
	if tag, ok := tags["Primary"]; ok {
		dto.AlbumPrimaryImageTag = apiutil.Ptr(tag)
	}
}

func byArtist(dto *api.BaseItemDto, artist *items.Item) {
	credited := []api.NameGuidPair{{Id: apiutil.Ptr(artist.ID), Name: apiutil.Ptr(artist.Name)}}
	dto.AlbumArtist = apiutil.Ptr(artist.Name)
	dto.AlbumArtists = apiutil.Ptr(credited)
	dto.ArtistItems = apiutil.Ptr(credited)
	dto.Artists = apiutil.Ptr([]string{artist.Name})
}
