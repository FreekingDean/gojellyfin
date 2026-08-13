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
}

func New(items *items.Service, libraries *libraries.Service, filesystem *filesystem.Service) *Scanner {
	return &Scanner{items: items, libraries: libraries, filesystem: filesystem}
}

func (s *Scanner) Scan(ctx context.Context, report jobs.Reporter) error {
	scanned, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return err
	}

	for index, library := range scanned {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.scanLibrary(ctx, library); err != nil {
			log.Printf("scan %s: %v", library.Name, err)
		}
		report(float64(index+1) / float64(len(scanned)) * 100)
	}

	return ctx.Err()
}

func (s *Scanner) scanLibrary(ctx context.Context, library *libraries.Library) error {
	seen := make([]string, 0)

	for _, location := range library.Locations {
		var paths []string
		var err error

		switch library.CollectionType {
		case librarymodal.CollectionTypeTvshows:
			paths, err = s.scanShows(ctx, library, location)
		case librarymodal.CollectionTypeMovies:
			paths, err = s.scanMovies(ctx, library, location)
		default:
			err = fmt.Errorf("unsupported collection type: %s", library.CollectionType)
		}
		if err != nil {
			return err
		}
		seen = append(seen, paths...)
	}

	log.Printf("scanned %s: %d items", library.Name, len(seen))

	return s.items.DeleteItemsNotInPaths(ctx, library.ID, seen)
}

func (s *Scanner) scanMovies(ctx context.Context, library *libraries.Library, root string) ([]string, error) {
	seen := make([]string, 0)

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if walkErr := ctx.Err(); walkErr != nil {
			return walkErr
		}
		if err != nil || entry.IsDir() || !isVideo(entry.Name()) {
			return nil
		}

		title := stripExtension(entry.Name())
		if parent := filepath.Dir(path); parent != root {
			title = filepath.Base(parent)
		}

		name, year := parseTitle(title)
		item, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:      library.ID,
			Kind:           itemmodal.KindMovie,
			Name:           name,
			SortName:       sortName(name),
			Path:           path,
			ProductionYear: year,
			DateModified:   modifiedAt(entry),
		})
		if err != nil {
			return err
		}
		seen = append(seen, path)

		if err := s.scanArtwork(ctx, item.ID, path, false); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}
		if parent := filepath.Dir(path); parent != root {
			if err := s.scanArtwork(ctx, item.ID, parent, true); err != nil {
				log.Printf("artwork %s: %v", parent, err)
			}
		}

		return s.scanMedia(ctx, item)
	})

	return seen, err
}

func (s *Scanner) scanShows(ctx context.Context, library *libraries.Library, root string) ([]string, error) {
	seen := make([]string, 0)

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}

		seriesPath := filepath.Join(root, entry.Name())
		name, year := parseTitle(entry.Name())
		series, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:      library.ID,
			Kind:           itemmodal.KindSeries,
			Name:           name,
			SortName:       sortName(name),
			Path:           seriesPath,
			ProductionYear: year,
			DateModified:   modifiedAt(entry),
		})
		if err != nil {
			return nil, err
		}
		seen = append(seen, seriesPath)

		if err := s.scanArtwork(ctx, series.ID, seriesPath, true); err != nil {
			log.Printf("artwork %s: %v", seriesPath, err)
		}

		paths, err := s.scanSeries(ctx, library, series.Name, series.ID, seriesPath)
		if err != nil {
			return nil, err
		}
		seen = append(seen, paths...)
	}

	return seen, nil
}

func (s *Scanner) scanSeries(ctx context.Context, library *libraries.Library, seriesName string, seriesID uuid.UUID, seriesPath string) ([]string, error) {
	seen := make([]string, 0)

	entries, err := os.ReadDir(seriesPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		path := filepath.Join(seriesPath, entry.Name())

		if !entry.IsDir() {
			if !isVideo(entry.Name()) {
				continue
			}
			if err := s.upsertEpisode(ctx, library, seriesName, seriesID, path, entry); err != nil {
				return nil, err
			}
			seen = append(seen, path)
			continue
		}

		number, ok := parseSeason(entry.Name())
		if !ok {
			continue
		}

		season, err := s.items.SaveScanned(ctx, items.Scanned{
			LibraryID:    library.ID,
			ParentID:     &seriesID,
			Kind:         itemmodal.KindSeason,
			Name:         seasonName(number),
			SortName:     seasonSortName(number),
			Path:         path,
			IndexNumber:  number,
			DateModified: modifiedAt(entry),
		})
		if err != nil {
			return nil, err
		}
		seen = append(seen, path)

		if err := s.scanArtwork(ctx, season.ID, path, true); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}

		episodes, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, episode := range episodes {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if episode.IsDir() || !isVideo(episode.Name()) {
				continue
			}

			episodePath := filepath.Join(path, episode.Name())
			if err := s.upsertEpisode(ctx, library, seriesName, season.ID, episodePath, episode); err != nil {
				return nil, err
			}
			seen = append(seen, episodePath)
		}
	}

	return seen, nil
}

func (s *Scanner) upsertEpisode(ctx context.Context, library *libraries.Library, seriesName string, parentID uuid.UUID, path string, entry os.DirEntry) error {
	season, number, title, ok := parseEpisode(entry.Name())
	if !ok {
		title = stripExtension(entry.Name())
	}
	if title == "" {
		title = episodeTitle(seriesName, season, number)
	}

	item, err := s.items.SaveScanned(ctx, items.Scanned{
		LibraryID:         library.ID,
		ParentID:          &parentID,
		Kind:              itemmodal.KindEpisode,
		Name:              title,
		SortName:          sortName(title),
		Path:              path,
		IndexNumber:       number,
		ParentIndexNumber: season,
		DateModified:      modifiedAt(entry),
	})
	if err != nil {
		return err
	}

	if err := s.scanArtwork(ctx, item.ID, path, false); err != nil {
		log.Printf("artwork %s: %v", path, err)
	}

	return s.scanMedia(ctx, item)
}

func (s *Scanner) scanMedia(ctx context.Context, item *items.Item) error {
	if err := s.probeMedia(ctx, item); err != nil {
		return err
	}

	if err := s.scanSubtitles(ctx, item); err != nil {
		log.Printf("subtitles %s: %v", item.Path, err)
	}

	return nil
}

func (s *Scanner) probeMedia(ctx context.Context, item *items.Item) error {
	if !ffmpeg.Available() {
		return nil
	}

	if err := s.probe(ctx, item); err != nil {
		log.Printf("probe %s: %v", item.Path, err)
	}

	return nil
}

func modifiedAt(entry os.DirEntry) time.Time {
	info, err := entry.Info()
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}
