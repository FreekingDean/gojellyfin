package collage

import (
	"context"
	"fmt"
	"image"
	"io"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/artwork"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
)

const (
	ContentType = "image/jpeg"
	Tag         = "collage"

	posterLimit = 16 << 20
	lifetime    = 15 * time.Minute
)

type held struct {
	body  []byte
	built time.Time
}

type Service struct {
	items      *items.Service
	filesystem *filesystem.Service
	artwork    artwork.Store

	building sync.Mutex
	mutex    sync.RWMutex
	cache    map[uuid.UUID]held
	now      func() time.Time
}

func New(records *items.Service, files *filesystem.Service, stored artwork.Store) *Service {
	return &Service{
		items:      records,
		filesystem: files,
		artwork:    stored,
		cache:      map[uuid.UUID]held{},
		now:        time.Now,
	}
}

func (s *Service) Image(ctx context.Context, libraryID uuid.UUID) ([]byte, bool) {
	if body, ok := s.cached(libraryID); ok {
		return body, true
	}

	s.building.Lock()
	defer s.building.Unlock()

	if body, ok := s.cached(libraryID); ok {
		return body, true
	}

	body, err := s.build(ctx, libraryID)
	if err != nil {
		log.Printf("collage %s: %v", libraryID, err)

		return nil, false
	}
	if len(body) == 0 {
		return nil, false
	}

	s.mutex.Lock()
	s.cache[libraryID] = held{body: body, built: s.now()}
	s.mutex.Unlock()

	return body, true
}

func (s *Service) cached(libraryID uuid.UUID) ([]byte, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	entry, ok := s.cache[libraryID]

	return entry.body, ok && s.now().Sub(entry.built) < lifetime
}

func (s *Service) build(ctx context.Context, libraryID uuid.UUID) ([]byte, error) {
	chosen, err := s.items.LibraryPosters(ctx, libraryID, cells)
	if err != nil {
		return nil, err
	}

	posters := make([]image.Image, 0, len(chosen))
	for _, record := range chosen {
		poster, err := s.decode(ctx, record)
		if err != nil {
			log.Printf("collage %s poster: %v", libraryID, err)

			continue
		}
		posters = append(posters, poster)
	}
	if len(posters) == 0 {
		return nil, nil
	}

	return compose(posters)
}

func (s *Service) decode(ctx context.Context, record *items.Image) (image.Image, error) {
	body, err := s.open(ctx, record)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()

	poster, _, err := image.Decode(io.LimitReader(body, posterLimit))
	if err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", record.Path, err)
	}

	return poster, nil
}

func (s *Service) open(ctx context.Context, record *items.Image) (io.ReadCloser, error) {
	if record.Source == items.ImageSourceLocal {
		body, _, err := s.filesystem.Open(ctx, record.Path)

		return body, err
	}

	body, _, found, err := s.artwork.Open(ctx, record.Path)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%s is not stored", record.Path)
	}

	return body, nil
}
