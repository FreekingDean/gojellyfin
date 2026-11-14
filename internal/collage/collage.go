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
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
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

type layout func([]image.Image) ([]byte, error)

type Libraries interface {
	LibraryPosters(ctx context.Context, libraryID uuid.UUID, limit int) ([]*items.Image, error)
	LibraryFresh(ctx context.Context, libraryID uuid.UUID, since time.Time) (int, error)
}

type Names interface {
	Library(ctx context.Context, id uuid.UUID) (*libraries.Library, error)
}

type Shelves interface {
	Card(ctx context.Context, id uuid.UUID, viewer items.Viewer, limit int) (shelves.Card, bool, error)
}

type plan struct {
	posters []*items.Image
	arrange layout
	caption string
}

type Service struct {
	libraries Libraries
	names     Names
	shelves   Shelves
	client    *http.Client
	now       func() time.Time

	flight singleflight.Group
	mutex  sync.RWMutex
	held   map[uuid.UUID]held
}

func New(records *items.Service, named *libraries.Service, shelved *shelves.Service) *Service {
	return newService(records, named, shelved, time.Now)
}

func newService(listing Libraries, named Names, shelved Shelves, now func() time.Time) *Service {
	return &Service{
		libraries: listing,
		names:     named,
		shelves:   shelved,
		now:       now,
		client:    &http.Client{Timeout: fetchTimeout},
		held:      map[uuid.UUID]held{},
	}
}

func (s *Service) Tag(ctx context.Context, id uuid.UUID) (string, error) {
	planned, err := s.plan(ctx, id)
	if err != nil {
		return "", err
	}

	return planned.fingerprint(), nil
}

func (s *Service) plan(ctx context.Context, id uuid.UUID) (plan, error) {
	card, found, err := s.shelves.Card(ctx, id, items.Everyone, stacked)
	if err != nil {
		return plan{}, err
	}
	if found {
		return titledPlan(card.Name, card.Fresh, card.Posters), nil
	}

	listed, err := s.libraries.LibraryPosters(ctx, id, stacked)
	if err != nil || len(listed) == 0 {
		return plan{}, err
	}

	library, err := s.names.Library(ctx, id)
	if err != nil {
		return plan{}, err
	}

	fresh, err := s.libraries.LibraryFresh(ctx, id, s.now().Add(-shelves.FreshWindow))
	if err != nil {
		return plan{}, err
	}

	return titledPlan(library.Name, fresh, listed), nil
}

func titledPlan(name string, fresh int, posters []*items.Image) plan {
	return plan{posters: posters, arrange: titled(name, fresh), caption: name + "\x00" + headline(fresh)}
}

func (s *Service) Image(ctx context.Context, id uuid.UUID) (Collage, bool) {
	planned, err := s.plan(ctx, id)
	if err != nil {
		log.Printf("collage %s: %v", id, err)

		return Collage{}, false
	}

	tag := planned.fingerprint()
	if tag == "" {
		return Collage{}, false
	}
	if body, ok := s.cached(id, tag); ok {
		return Collage{Body: body, Complete: true}, true
	}

	built, err, _ := s.flight.Do(id.String()+":"+tag, func() (any, error) {
		return s.build(context.WithoutCancel(ctx), id, tag, planned)
	})
	if err != nil {
		log.Printf("collage %s: %v", id, err)

		return Collage{}, false
	}

	answer, ok := built.(Collage)

	return answer, ok && len(answer.Body) > 0
}

func (s *Service) cached(id uuid.UUID, tag string) ([]byte, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	entry, ok := s.held[id]

	return entry.body, ok && entry.tag == tag
}

func (s *Service) build(ctx context.Context, id uuid.UUID, tag string, planned plan) (Collage, error) {
	posters := planned.posters
	decoded := make([]image.Image, len(posters))

	var group errgroup.Group
	for index, poster := range posters {
		group.Go(func() error {
			picture, err := s.fetch(ctx, poster.URL)
			if err != nil {
				log.Printf("collage %s poster: %v", id, err)

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

	body, err := planned.arrange(fetched)
	if err != nil {
		return Collage{}, err
	}

	complete := len(fetched) == len(posters)
	if complete {
		s.mutex.Lock()
		s.held[id] = held{tag: tag, body: body}
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

func (p plan) fingerprint() string {
	if len(p.posters) == 0 {
		return ""
	}

	hash := sha1.New()
	_, _ = io.WriteString(hash, p.caption)
	for _, poster := range p.posters {
		_, _ = io.WriteString(hash, poster.Tag)
		_, _ = hash.Write([]byte{0})
	}

	return hex.EncodeToString(hash.Sum(nil))
}
