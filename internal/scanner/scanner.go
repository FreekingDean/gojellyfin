package scanner

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

const collectionTypeTVShows = "tvshows"

type Scanner struct {
	items     *items.Service
	libraries *libraries.Service

	mu      sync.Mutex
	running bool
}

func New(items *items.Service, libraries *libraries.Service) *Scanner {
	return &Scanner{items: items, libraries: libraries}
}

func (s *Scanner) Scan(ctx context.Context) error {
	if !s.start() {
		return nil
	}
	defer s.finish()

	libraries, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return err
	}

	for _, library := range libraries {
		if err := s.scanLibrary(ctx, &library); err != nil {
			log.Printf("scan %s: %v", library.Name, err)
		}
	}

	return nil
}

func (s *Scanner) start() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return false
	}
	s.running = true

	return true
}

func (s *Scanner) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
}

func (s *Scanner) scanLibrary(ctx context.Context, library *libraries.Library) error {
	seen := make([]string, 0)

	for _, path := range library.Paths {
		var paths []string
		var err error

		switch library.CollectionType {
		case collectionTypeTVShows:
			paths, err = s.scanShows(ctx, library, path.Path)
		default:
			paths, err = s.scanMovies(ctx, library, path.Path)
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
		if err != nil || entry.IsDir() || !isVideo(entry.Name()) {
			return nil
		}

		title := stripExtension(entry.Name())
		if parent := filepath.Dir(path); parent != root {
			title = filepath.Base(parent)
		}

		name, year := parseTitle(title)
		item := &items.Item{
			LibraryID:      library.ID,
			Type:           "Movie",
			Name:           name,
			SortName:       sortName(name),
			Path:           path,
			ProductionYear: year,
			DateModified:   modifiedAt(entry),
		}
		if err := s.items.UpsertItem(ctx, item); err != nil {
			return err
		}
		seen = append(seen, path)

		id, err := s.itemID(ctx, library.ID, path)
		if err != nil {
			return err
		}
		if err := s.scanArtwork(ctx, id, path, false); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}
		if parent := filepath.Dir(path); parent != root {
			if err := s.scanArtwork(ctx, id, parent, true); err != nil {
				log.Printf("artwork %s: %v", parent, err)
			}
		}

		return s.probeMedia(ctx, library.ID, path)
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
		if !entry.IsDir() {
			continue
		}

		seriesPath := filepath.Join(root, entry.Name())
		name, year := parseTitle(entry.Name())
		series := &items.Item{
			LibraryID:      library.ID,
			Type:           "Series",
			Name:           name,
			SortName:       sortName(name),
			Path:           seriesPath,
			ProductionYear: year,
			DateModified:   modifiedAt(entry),
		}
		if err := s.items.UpsertItem(ctx, series); err != nil {
			return nil, err
		}
		seen = append(seen, seriesPath)

		seriesID, err := s.itemID(ctx, library.ID, seriesPath)
		if err != nil {
			return nil, err
		}
		if err := s.scanArtwork(ctx, seriesID, seriesPath, true); err != nil {
			log.Printf("artwork %s: %v", seriesPath, err)
		}

		paths, err := s.scanSeries(ctx, library, series.Name, seriesID, seriesPath)
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

		season := &items.Item{
			LibraryID:    library.ID,
			ParentID:     &seriesID,
			Type:         "Season",
			Name:         seasonName(number),
			SortName:     seasonSortName(number),
			Path:         path,
			IndexNumber:  number,
			DateModified: modifiedAt(entry),
		}
		if err := s.items.UpsertItem(ctx, season); err != nil {
			return nil, err
		}
		seen = append(seen, path)

		seasonID, err := s.itemID(ctx, library.ID, path)
		if err != nil {
			return nil, err
		}
		if err := s.scanArtwork(ctx, seasonID, path, true); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}

		episodes, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, episode := range episodes {
			if episode.IsDir() || !isVideo(episode.Name()) {
				continue
			}

			episodePath := filepath.Join(path, episode.Name())
			if err := s.upsertEpisode(ctx, library, seriesName, seasonID, episodePath, episode); err != nil {
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

	err := s.items.UpsertItem(ctx, &items.Item{
		LibraryID:         library.ID,
		ParentID:          &parentID,
		Type:              "Episode",
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

	if id, err := s.itemID(ctx, library.ID, path); err == nil {
		if err := s.scanArtwork(ctx, id, path, false); err != nil {
			log.Printf("artwork %s: %v", path, err)
		}
	}

	return s.probeMedia(ctx, library.ID, path)
}

func (s *Scanner) probeMedia(ctx context.Context, libraryID uuid.UUID, path string) error {
	if !ffmpeg.Available() {
		return nil
	}

	if err := s.probe(ctx, libraryID, path); err != nil {
		log.Printf("probe %s: %v", path, err)
	}

	return nil
}

func (s *Scanner) itemID(ctx context.Context, libraryID uuid.UUID, path string) (uuid.UUID, error) {
	item, err := s.items.GetItemByPath(ctx, libraryID, path)
	if err != nil {
		return uuid.Nil, err
	}

	return item.ID, nil
}

func modifiedAt(entry os.DirEntry) time.Time {
	info, err := entry.Info()
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}
