package items

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	genremodel "github.com/FreekingDean/gojellyfin/internal/store/genre"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	playlistmodel "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/playlistentry"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
)

const mixLength = 200

type mixSeed struct {
	artists []uuid.UUID
	genres  []uuid.UUID
}

func (s *Service) Mix(ctx context.Context, viewer Viewer, itemID uuid.UUID, limit int) ([]*Item, error) {
	record, err := s.ItemByID(ctx, viewer, itemID)
	if store.IsNotFound(err) {
		return nil, fmt.Errorf("%w: no item %s", ErrNotFound, itemID)
	}
	if err != nil {
		return nil, err
	}

	seed := mixSeed{}
	if record.Kind == itemmodel.KindPlaylist {
		entries, err := s.store.Item.Query().
			Where(itemmodel.HasPlaylistEntriesWith(entrymodel.HasPlaylistWith(playlistmodel.ItemID(record.ID)))).
			AllModels(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read the playlist: %w", err)
		}
		for _, entry := range entries {
			if err := s.seedFrom(ctx, entry, &seed); err != nil {
				return nil, err
			}
		}
	} else if err := s.seedFrom(ctx, record, &seed); err != nil {
		return nil, err
	}

	return s.mixOf(ctx, viewer, seed, limit)
}

func (s *Service) GenreMix(ctx context.Context, viewer Viewer, genreID *uuid.UUID, name string, limit int) ([]*Item, error) {
	which := genremodel.NameEqualFold(name)
	if genreID != nil {
		which = genremodel.ID(*genreID)
	}

	ids, err := s.store.Genre.Query().Where(which).IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find the genre: %w", err)
	}

	return s.mixOf(ctx, viewer, mixSeed{genres: ids}, limit)
}

func (s *Service) seedFrom(ctx context.Context, record *Item, seed *mixSeed) error {
	lineage := []*Item{record}
	for current := record; current.Kind != itemmodel.KindMusicArtist && current.ParentID != nil; {
		parent, err := s.ItemByID(ctx, Everyone, *current.ParentID)
		if err != nil {
			return err
		}
		lineage = append(lineage, parent)
		current = parent
	}

	for _, ancestor := range lineage {
		if ancestor.Kind == itemmodel.KindMusicArtist && !slices.Contains(seed.artists, ancestor.ID) {
			seed.artists = append(seed.artists, ancestor.ID)
		}
		if ancestor.Kind != itemmodel.KindMusicArtist && ancestor.Kind != itemmodel.KindMusicAlbum {
			continue
		}

		genres, err := s.store.Genre.Query().Where(genremodel.HasItemsWith(itemmodel.ID(ancestor.ID))).IDs(ctx)
		if err != nil {
			return fmt.Errorf("failed to read genres: %w", err)
		}
		for _, genre := range genres {
			if !slices.Contains(seed.genres, genre) {
				seed.genres = append(seed.genres, genre)
			}
		}
	}

	return nil
}

func (s *Service) mixOf(ctx context.Context, viewer Viewer, seed mixSeed, limit int) ([]*Item, error) {
	if len(seed.artists) == 0 && len(seed.genres) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = mixLength
	}

	album := []predicate.Item{itemmodel.ParentIDIn(seed.artists...)}
	if len(seed.genres) > 0 {
		album = append(album,
			itemmodel.HasGenresWith(genremodel.IDIn(seed.genres...)),
			itemmodel.HasParentWith(itemmodel.HasGenresWith(genremodel.IDIn(seed.genres...))),
		)
	}

	tracks, err := s.query(viewer).
		Where(itemmodel.KindEQ(itemmodel.KindAudio), itemmodel.HasParentWith(itemmodel.Or(album...))).
		Order(orderRandom).
		Limit(limit).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to mix: %w", err)
	}

	return tracks, nil
}
