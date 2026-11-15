package cards

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
	"github.com/FreekingDean/gojellyfin/internal/users"
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

type Picture struct {
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
	Card(ctx context.Context, id uuid.UUID, viewer items.Viewer, userID uuid.UUID, limit int) (shelves.Card, bool, error)
}

type Access interface {
	Access(ctx context.Context, id uuid.UUID) (users.Access, error)
}

type plan struct {
	owner   uuid.UUID
	posters []*items.Image
	arrange layout
	caption string
}

type Service struct {
	libraries Libraries
	names     Names
	shelves   Shelves
	access    Access
	client    *http.Client
	now       func() time.Time

	flight singleflight.Group
	mutex  sync.RWMutex
	held   map[key]held
}

type key struct {
	id    uuid.UUID
	owner uuid.UUID
}

func New(records *items.Service, named *libraries.Service, shelved *shelves.Service, access *users.Service) *Service {
	return newService(records, named, shelved, access, time.Now)
}

func newService(listing Libraries, named Names, shelved Shelves, access Access, now func() time.Time) *Service {
	return &Service{
		libraries: listing,
		names:     named,
		shelves:   shelved,
		access:    access,
		now:       now,
		client:    &http.Client{Timeout: fetchTimeout, Transport: tracing.Transport(nil)},
		held:      map[key]held{},
	}
}

func (s *Service) Tag(ctx context.Context, id, userID uuid.UUID) (string, error) {
	planned, err := s.plan(ctx, id, userID)
	if err != nil {
		return "", err
	}

	return planned.tag(), nil
}

func (s *Service) plan(ctx context.Context, id, userID uuid.UUID) (plan, error) {
	card, found, err := s.shelves.Card(ctx, id, items.Everyone, uuid.Nil, stacked)
	if err != nil {
		return plan{}, err
	}
	if !found && userID != uuid.Nil {
		card, found, err = s.personal(ctx, id, userID)
		if err != nil {
			return plan{}, err
		}
	}
	if found {
		planned := titledPlan(card.Name, card.Fresh, card.Posters)
		if card.Personal {
			planned.owner = userID
		}

		return planned, nil
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

func (s *Service) personal(ctx context.Context, id, userID uuid.UUID) (shelves.Card, bool, error) {
	access, err := s.access.Access(ctx, userID)
	if err != nil {
		return shelves.Card{}, false, err
	}

	return s.shelves.Card(ctx, id, items.Viewer{All: access.All, Libraries: access.Libraries}, userID, stacked)
}

func titledPlan(name string, fresh int, posters []*items.Image) plan {
	return plan{posters: posters, arrange: titled(name, fresh), caption: name + "\x00" + headline(fresh)}
}

func (s *Service) Image(ctx context.Context, id uuid.UUID, asked string) (Picture, bool) {
	owner := ownerOf(asked)
	planned, err := s.plan(ctx, id, owner)
	if err != nil {
		log.Printf("card %s: %v", id, err)

		return Picture{}, false
	}

	tag := planned.tag()
	if tag == "" || planned.owner != owner || owner != uuid.Nil && tag != asked {
		return Picture{}, false
	}

	held := key{id: id, owner: owner}
	if body, ok := s.cached(held, tag); ok {
		return Picture{Body: body, Complete: true}, true
	}

	built, err, _ := s.flight.Do(id.String()+":"+tag, func() (any, error) {
		return s.build(context.WithoutCancel(ctx), held, tag, planned)
	})
	if err != nil {
		log.Printf("card %s: %v", id, err)

		return Picture{}, false
	}

	answer, ok := built.(Picture)

	return answer, ok && len(answer.Body) > 0
}

func ownerOf(tag string) uuid.UUID {
	prefix, _, personal := strings.Cut(tag, "-")
	if !personal {
		return uuid.Nil
	}

	owner, err := uuid.Parse(prefix)
	if err != nil {
		return uuid.Nil
	}

	return owner
}

func (s *Service) cached(held key, tag string) ([]byte, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	entry, ok := s.held[held]

	return entry.body, ok && entry.tag == tag
}

func (s *Service) build(ctx context.Context, slot key, tag string, planned plan) (Picture, error) {
	id := slot.id
	posters := planned.posters
	decoded := make([]image.Image, len(posters))

	var group errgroup.Group
	for index, poster := range posters {
		group.Go(func() error {
			picture, err := s.fetch(ctx, poster.URL)
			if err != nil {
				log.Printf("card %s poster: %v", id, err)

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
		return Picture{}, nil
	}

	body, err := planned.arrange(fetched)
	if err != nil {
		return Picture{}, err
	}

	complete := len(fetched) == len(posters)
	if complete {
		s.mutex.Lock()
		s.held[slot] = held{tag: tag, body: body}
		s.mutex.Unlock()
	}

	return Picture{Body: body, Complete: complete}, nil
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

func (p plan) tag() string {
	fingerprint := p.fingerprint()
	if fingerprint == "" || p.owner == uuid.Nil {
		return fingerprint
	}

	return hex.EncodeToString(p.owner[:]) + "-" + fingerprint
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
