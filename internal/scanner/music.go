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

func (s *Scanner) scanMusic(ctx context.Context, library *libraries.Library, root string, found *walk) error {
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
			if err := s.upsertTrack(ctx, library, nil, nil, path, entry); err != nil {
				return err
			}
			found.found(path)
			continue
		}

		name := clean(entry.Name())
		artist, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:    library.ID,
			Kind:         itemmodal.KindMusicArtist,
			Name:         name,
			SortName:     sortName(name),
			Path:         path,
			DateModified: modifiedAt(entry),
		})
		if err != nil {
			return err
		}
		found.found(path)

		if err := s.scanArtwork(ctx, artist.ID, path, true); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}

		if err := s.scanArtist(ctx, library, artist.ID, path, found); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scanner) scanArtist(ctx context.Context, library *libraries.Library, artistID uuid.UUID, artistPath string, found *walk) error {
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
			if err := s.upsertTrack(ctx, library, &artistID, nil, path, entry); err != nil {
				return err
			}
			found.found(path)
			continue
		}

		name, year := parseTitle(entry.Name())
		album, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:      library.ID,
			ParentID:       &artistID,
			Kind:           itemmodal.KindMusicAlbum,
			Name:           name,
			SortName:       sortName(name),
			Path:           path,
			ProductionYear: year,
			DateModified:   modifiedAt(entry),
		})
		if err != nil {
			return err
		}
		found.found(path)

		if err := s.scanArtwork(ctx, album.ID, path, true); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}

		if err := s.scanAlbum(ctx, library, album.ID, path, nil, found); err != nil {
			return err
		}
	}

	return nil
}

// A disc directory carries its number down to the tracks and is the only
// nesting an album recurses into, so a stray folder cannot walk forever.
func (s *Scanner) scanAlbum(ctx context.Context, library *libraries.Library, albumID uuid.UUID, albumPath string, disc *int32, found *walk) error {
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
			if err := s.scanAlbum(ctx, library, albumID, path, number, found); err != nil {
				return err
			}
			continue
		}

		if !isAudio(entry.Name()) {
			continue
		}
		if err := s.upsertTrack(ctx, library, &albumID, disc, path, entry); err != nil {
			return err
		}
		found.found(path)
	}

	return nil
}

func (s *Scanner) upsertTrack(ctx context.Context, library *libraries.Library, parentID *uuid.UUID, disc *int32, path string, entry os.DirEntry) error {
	side, number, title := parseTrack(entry.Name())
	if side == nil {
		side = disc
	}

	item, err := s.items.SaveScanned(ctx, items.Scanned{
		LibraryID:         library.ID,
		ParentID:          parentID,
		Kind:              itemmodal.KindAudio,
		Name:              title,
		SortName:          sortName(title),
		Path:              path,
		IndexNumber:       number,
		ParentIndexNumber: side,
		DateModified:      modifiedAt(entry),
	})
	if err != nil {
		return err
	}

	if err := s.scanArtwork(ctx, item.ID, path, false); err != nil {
		log.Printf("artwork %s: %v", path, err)
	}

	return s.probeMedia(ctx, item)
}
