package scanner

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (s *Scanner) scanMusic(ctx context.Context, library *libraries.Library, root string, found *seen) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(root, entry.Name())

		if !entry.IsDir() {
			if !isAudio(entry.Name()) {
				continue
			}
			if err := s.scanTrack(ctx, library, nil, "", nil, path, entry, found); err != nil {
				return err
			}
			continue
		}

		name := clean(entry.Name())
		slug := titleSlug(name, nil)
		artist, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:    library.ID,
			Kind:         itemmodal.KindMusicArtist,
			Key:          musicArtistKey(slug),
			Name:         name,
			SortName:     sortName(name),
			DateModified: modifiedAt(entry),
		})
		if err != nil {
			return err
		}
		found.title(artist)

		if err := s.scanArtwork(ctx, artist.ID, path, true); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}

		if err := s.scanArtist(ctx, library, artist.ID, slug, path, found); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scanner) scanArtist(ctx context.Context, library *libraries.Library, artistID uuid.UUID, artist, artistPath string, found *seen) error {
	entries, err := os.ReadDir(artistPath)
	if err != nil {
		found.skip(artistPath, err)

		return nil
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(artistPath, entry.Name())

		if !entry.IsDir() {
			if !isAudio(entry.Name()) {
				continue
			}
			if err := s.scanTrack(ctx, library, &artistID, artist, nil, path, entry, found); err != nil {
				return err
			}
			continue
		}

		name, year := parseTitle(entry.Name())
		slug := albumSlug(artist, name, year)
		album, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:      library.ID,
			ParentID:       &artistID,
			Kind:           itemmodal.KindMusicAlbum,
			Key:            musicAlbumKey(slug),
			Name:           name,
			SortName:       sortName(name),
			ProductionYear: year,
			DateModified:   modifiedAt(entry),
		})
		if err != nil {
			return err
		}
		found.title(album)

		if err := s.scanArtwork(ctx, album.ID, path, true); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}

		if err := s.scanAlbum(ctx, library, album.ID, slug, path, nil, found); err != nil {
			return err
		}
	}

	return nil
}

// A disc directory carries its number down to the tracks and is the only
// nesting an album recurses into, so a stray folder cannot walk forever.
func (s *Scanner) scanAlbum(ctx context.Context, library *libraries.Library, albumID uuid.UUID, album, albumPath string, disc *int32, found *seen) error {
	entries, err := os.ReadDir(albumPath)
	if err != nil {
		found.skip(albumPath, err)

		return nil
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(albumPath, entry.Name())

		if entry.IsDir() {
			number, ok := parseDisc(entry.Name())
			if !ok || disc != nil {
				continue
			}
			if err := s.scanAlbum(ctx, library, albumID, album, path, number, found); err != nil {
				return err
			}
			continue
		}

		if !isAudio(entry.Name()) {
			continue
		}
		if err := s.scanTrack(ctx, library, &albumID, album, disc, path, entry, found); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scanner) scanTrack(ctx context.Context, library *libraries.Library, parentID *uuid.UUID, scope string, disc *int32, path string, entry os.DirEntry, found *seen) error {
	side, number, title := parseTrack(entry.Name())
	if side == nil {
		side = disc
	}

	item, err := s.scanFile(ctx, library, path, entry, found, items.Scanned{
		LibraryID:         library.ID,
		ParentID:          parentID,
		Kind:              itemmodal.KindAudio,
		Key:               audioKey(scope, side, number, title),
		Name:              title,
		SortName:          sortName(title),
		IndexNumber:       number,
		ParentIndexNumber: side,
	})
	if err != nil {
		return err
	}

	if err := s.scanArtwork(ctx, item.ID, path, false); err != nil {
		log.Printf("artwork %s: %v", path, err)
	}

	return nil
}
