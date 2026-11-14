package collage

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

const (
	ContentType = "image/jpeg"

	posterLimit  = 8 << 20
	fetchTimeout = 10 * time.Second
)

type held struct {
	tag  string
	body []byte
}

type Collage struct {
	Body     []byte
	Complete bool
}

type Service struct {
	items  *items.Service
	client *http.Client

	flight singleflight.Group
	mutex  sync.RWMutex
	held   map[uuid.UUID]held
}

func New(records *items.Service) *Service {
	return &Service{
		items:  records,
		client: &http.Client{Timeout: fetchTimeout},
		held:   map[uuid.UUID]held{},
	}
}

func (s *Service) Tag(ctx context.Context, libraryID uuid.UUID) (string, error) {
	posters, err := s.items.LibraryPosters(ctx, libraryID, cells)
	if err != nil {
		return "", err
	}

	return fingerprint(posters), nil
}

func (s *Service) Image(ctx context.Context, libraryID uuid.UUID) (Collage, bool) {
	posters, err := s.items.LibraryPosters(ctx, libraryID, cells)
	if err != nil {
		log.Printf("collage %s: %v", libraryID, err)

		return Collage{}, false
	}

	tag := fingerprint(posters)
	if tag == "" {
		return Collage{}, false
	}
	if body, ok := s.cached(libraryID, tag); ok {
		return Collage{Body: body, Complete: true}, true
	}

	built, err, _ := s.flight.Do(libraryID.String()+":"+tag, func() (any, error) {
		return s.build(context.WithoutCancel(ctx), libraryID, tag, posters)
	})
	if err != nil {
		log.Printf("collage %s: %v", libraryID, err)

		return Collage{}, false
	}

	answer, ok := built.(Collage)

	return answer, ok && len(answer.Body) > 0
}

func (s *Service) cached(libraryID uuid.UUID, tag string) ([]byte, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	entry, ok := s.held[libraryID]

	return entry.body, ok && entry.tag == tag
}

func (s *Service) build(ctx context.Context, libraryID uuid.UUID, tag string, posters []*items.Image) (Collage, error) {
	decoded := make([]image.Image, len(posters))

	var group errgroup.Group
	for index, poster := range posters {
		group.Go(func() error {
			picture, err := s.fetch(ctx, poster.URL)
			if err != nil {
				log.Printf("collage %s poster: %v", libraryID, err)

				return nil
			}
			decoded[index] = picture

			return nil
		})
	}
	_ = group.Wait()

	fetched := make([]image.Image, 0, len(decoded))
	for _, picture := range decoded {
		if picture != nil {
			fetched = append(fetched, picture)
		}
	}
	if len(fetched) == 0 {
		return Collage{}, nil
	}

	body, err := compose(fetched)
	if err != nil {
		return Collage{}, err
	}

	complete := len(fetched) == len(posters)
	if complete {
		s.mutex.Lock()
		s.held[libraryID] = held{tag: tag, body: body}
		s.mutex.Unlock()
	}

	return Collage{Body: body, Complete: complete}, nil
}

func (s *Service) fetch(ctx context.Context, url string) (image.Image, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build a request for %s: %w", url, err)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s: %s", url, response.Status)
	}

	picture, _, err := image.Decode(io.LimitReader(response.Body, posterLimit))
	if err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", url, err)
	}

	return picture, nil
}

func fingerprint(posters []*items.Image) string {
	if len(posters) == 0 {
		return ""
	}

	hash := sha1.New()
	for _, poster := range posters {
		_, _ = io.WriteString(hash, poster.Tag)
		_, _ = hash.Write([]byte{0})
	}

	return hex.EncodeToString(hash.Sum(nil))
}
