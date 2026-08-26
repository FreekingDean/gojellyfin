package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
)

type Scanner struct {
	items      *items.Service
	libraries  *libraries.Service
	filesystem *filesystem.Service
	ffmpeg     *ffmpeg.FFMpeg
}

func New(items *items.Service, libraries *libraries.Service, filesystem *filesystem.Service, ffmpeg *ffmpeg.FFMpeg) *Scanner {
	return &Scanner{items: items, libraries: libraries, filesystem: filesystem, ffmpeg: ffmpeg}
}

type seen struct {
	keys       []string
	paths      []string
	unreadable int
}

func (s *seen) title(item *items.Item) {
	s.keys = append(s.keys, item.Key)
}

func (s *seen) file(path string) {
	s.paths = append(s.paths, path)
}

func (s *seen) skip(path string, err error) {
	s.unreadable++
	log.Printf("skipping %s: %v", path, err)
}

func (s *seen) complete() bool {
	return s.unreadable == 0
}

func (s *Scanner) scanLibrary(ctx context.Context, library *libraries.Library) error {
	found := &seen{}

	if err := s.rekeyLegacy(ctx, library); err != nil {
		return err
	}

	for _, location := range library.Locations {
		var err error

		switch library.CollectionType {
		case librarymodal.CollectionTypeTvshows:
			err = s.scanShows(ctx, library, location, found)
		case librarymodal.CollectionTypeMovies:
			err = s.scanMovies(ctx, library, location, found)
		case librarymodal.CollectionTypeMusic:
			err = s.scanMusic(ctx, library, location, found)
		default:
			err = fmt.Errorf("unsupported collection type: %s", library.CollectionType)
		}
		if err != nil {
			return err
		}
	}

	log.Printf("scanned %s: %d items, %d files", library.Name, len(found.keys), len(found.paths))

	if !found.complete() {
		log.Printf("not sweeping %s: %d directories could not be read", library.Name, found.unreadable)

		return nil
	}

	if err := s.items.DeleteItemsNotInKeys(ctx, library.ID, found.keys); err != nil {
		return err
	}

	return s.items.DeleteSourcesNotInPaths(ctx, library.ID, found.paths)
}

func (s *Scanner) scanMovies(ctx context.Context, library *libraries.Library, root string, found *seen) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if walkErr := ctx.Err(); walkErr != nil {
			return walkErr
		}
		if err != nil {
			if path == root {
				return err
			}
			found.skip(path, err)

			return nil
		}
		if entry.IsDir() || !isVideo(entry.Name()) {
			return nil
		}

		title := stripExtension(entry.Name())
		if parent := filepath.Dir(path); parent != root {
			title = filepath.Base(parent)
		}

		name, year := parseTitle(title)
		item, err := s.scanFile(ctx, library, path, entry, found, items.Scanned{
			LibraryID:      library.ID,
			Kind:           itemmodal.KindMovie,
			Key:            movieKey(name, year),
			Name:           name,
			SortName:       sortName(name),
			ProductionYear: year,
		})
		if err != nil {
			return err
		}

		if err := s.scanArtwork(ctx, item.ID, path, false); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}
		if parent := filepath.Dir(path); parent != root {
			if err := s.scanArtwork(ctx, item.ID, parent, true); err != nil {
				log.Printf("artwork %s: %v", parent, err)
			}
		}

		return nil
	})
}

func (s *Scanner) scanShows(ctx context.Context, library *libraries.Library, root string, found *seen) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			continue
		}

		seriesPath := filepath.Join(root, entry.Name())
		name, year := parseTitle(entry.Name())
		slug := titleSlug(name, year)
		series, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:      library.ID,
			Kind:           itemmodal.KindSeries,
			Key:            seriesKey(slug),
			Name:           name,
			SortName:       sortName(name),
			ProductionYear: year,
			DateModified:   modifiedAt(entry),
		})
		if err != nil {
			return err
		}
		found.title(series)

		if err := s.scanArtwork(ctx, series.ID, seriesPath, true); err != nil {
			log.Printf("artwork %s: %v", seriesPath, err)
		}

		if err := s.scanSeries(ctx, library, series, slug, seriesPath, found); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scanner) scanSeries(ctx context.Context, library *libraries.Library, series *items.Item, slug, seriesPath string, found *seen) error {
	entries, err := os.ReadDir(seriesPath)
	if err != nil {
		found.skip(seriesPath, err)

		return nil
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(seriesPath, entry.Name())

		if !entry.IsDir() {
			if !isVideo(entry.Name()) {
				continue
			}
			if err := s.scanEpisode(ctx, library, series, series.ID, slug, path, entry, found); err != nil {
				return err
			}
			continue
		}

		number, ok := parseSeason(entry.Name())
		if !ok {
			continue
		}

		season, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:    library.ID,
			ParentID:     &series.ID,
			Kind:         itemmodal.KindSeason,
			Key:          seasonKey(slug, number),
			Name:         seasonName(number),
			SortName:     seasonSortName(number),
			IndexNumber:  number,
			DateModified: modifiedAt(entry),
		})
		if err != nil {
			return err
		}
		found.title(season)

		if err := s.scanArtwork(ctx, season.ID, path, true); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}

		episodes, err := os.ReadDir(path)
		if err != nil {
			found.skip(path, err)

			continue
		}
		for _, episode := range episodes {
			if err := ctx.Err(); err != nil {
				return err
			}
			if episode.IsDir() || !isVideo(episode.Name()) {
				continue
			}

			episodePath := filepath.Join(path, episode.Name())
			if err := s.scanEpisode(ctx, library, series, season.ID, slug, episodePath, episode, found); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Scanner) scanEpisode(ctx context.Context, library *libraries.Library, series *items.Item, parentID uuid.UUID, slug, path string, entry os.DirEntry, found *seen) error {
	season, number, title, ok := parseEpisode(entry.Name())
	if !ok {
		title = stripExtension(entry.Name())
	}
	if title == "" {
		title = episodeTitle(series.Name, season, number)
	}

	item, err := s.scanFile(ctx, library, path, entry, found, items.Scanned{
		LibraryID:         library.ID,
		ParentID:          &parentID,
		Kind:              itemmodal.KindEpisode,
		Key:               episodeKey(slug, season, number, title),
		Name:              title,
		SortName:          sortName(title),
		IndexNumber:       number,
		ParentIndexNumber: season,
	})
	if err != nil {
		return err
	}

	if err := s.scanArtwork(ctx, item.ID, path, false); err != nil {
		log.Printf("artwork %s: %v", path, err)
	}

	return nil
}

func (s *Scanner) scanFile(ctx context.Context, library *libraries.Library, path string, entry os.DirEntry, found *seen, scanned items.Scanned) (*items.Item, error) {
	modified := modifiedAt(entry)
	scanned.DateModified = modified

	jobs.Heartbeat(ctx, path)

	item, err := s.items.SaveScanned(ctx, scanned)
	if err != nil {
		return nil, err
	}
	found.title(item)
	found.file(path)

	source, err := s.items.SaveSource(ctx, items.ScannedSource{
		LibraryID:    library.ID,
		ItemID:       item.ID,
		Path:         path,
		Name:         filepath.Base(path),
		DateModified: modified,
	})
	if err != nil {
		return nil, err
	}

	if err := s.scanSubtitles(ctx, item.ID, source); err != nil {
		log.Printf("subtitles %s: %v", path, err)
	}

	return item, nil
}

func modifiedAt(entry os.DirEntry) time.Time {
	info, err := entry.Info()
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}
