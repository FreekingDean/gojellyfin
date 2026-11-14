package discover

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const (
	trendingFor = time.Hour
	detailFor   = 24 * time.Hour
	detailLimit = 4096
	providerKey = "Tmdb"
)

type Provider interface {
	Enabled() bool
	Movie(ctx context.Context, tmdbID int) (items.Metadata, bool, error)
	Series(ctx context.Context, tmdbID int) (items.Metadata, bool, error)
	Trending(ctx context.Context, kind items.Kind) ([]items.Metadata, error)
}

type Library interface {
	HeldKeys(ctx context.Context, keys []string) ([]string, error)
}

type Title struct {
	ID       uuid.UUID
	Kind     items.Kind
	TmdbID   uint32
	Key      string
	Metadata items.Metadata
}

type listing struct {
	titles  []Title
	fetched time.Time
}

type detail struct {
	title    Title
	complete bool
	fetched  time.Time
}

type Service struct {
	provider Provider
	library  Library
	now      func() time.Time

	mutex    sync.Mutex
	trending map[items.Kind]listing
	details  map[uuid.UUID]detail
}

func New(provider Provider, library *items.Service) *Service {
	return newService(provider, library, time.Now)
}

func newService(provider Provider, library Library, now func() time.Time) *Service {
	return &Service{
		provider: provider,
		library:  library,
		now:      now,
		trending: map[items.Kind]listing{},
		details:  map[uuid.UUID]detail{},
	}
}

func (s *Service) Enabled() bool {
	return s.provider.Enabled()
}

func (s *Service) Trending(ctx context.Context, kind items.Kind, limit int) ([]Title, error) {
	listed, err := s.listed(ctx, kind)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(listed))
	for _, title := range listed {
		keys = append(keys, title.Key)
	}

	held, err := s.library.HeldKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	owned := make(map[string]bool, len(held))
	for _, key := range held {
		owned[key] = true
	}

	unowned := make([]Title, 0, len(listed))
	for _, title := range listed {
		if owned[title.Key] {
			continue
		}
		if limit > 0 && len(unowned) == limit {
			break
		}
		unowned = append(unowned, title)
	}

	return unowned, nil
}

func (s *Service) listed(ctx context.Context, kind items.Kind) ([]Title, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	held, cached := s.trending[kind]
	if cached && s.now().Sub(held.fetched) < trendingFor {
		return held.titles, nil
	}

	answered, err := s.provider.Trending(ctx, kind)
	if err != nil {
		if cached {
			log.Printf("discover: serving the last trending %s list, the provider did not answer: %v", kind, err)

			return held.titles, nil
		}

		return nil, fmt.Errorf("failed to read what is trending: %w", err)
	}

	titles := make([]Title, 0, len(answered))
	for _, metadata := range answered {
		title, ok := titled(kind, metadata)
		if !ok {
			continue
		}
		titles = append(titles, title)
		if known, seen := s.details[title.ID]; !seen || !known.complete {
			s.remember(title, false)
		}
	}
	s.trending[kind] = listing{titles: titles, fetched: s.now()}

	return titles, nil
}

func titled(kind items.Kind, metadata items.Metadata) (Title, bool) {
	if metadata.ProviderIds == nil {
		return Title{}, false
	}

	parsed, err := strconv.ParseUint((*metadata.ProviderIds)[providerKey], 10, 31)
	if err != nil || parsed == 0 {
		return Title{}, false
	}
	tmdbID := uint32(parsed)

	return Title{ID: ID(kind, tmdbID), Kind: kind, TmdbID: tmdbID, Key: Key(kind, tmdbID), Metadata: metadata}, true
}

func (s *Service) remember(title Title, complete bool) {
	if len(s.details) >= detailLimit {
		s.details = map[uuid.UUID]detail{}
	}
	s.details[title.ID] = detail{title: title, complete: complete, fetched: s.now()}
}

func (s *Service) Summary(ctx context.Context, id uuid.UUID) (Title, bool, error) {
	return s.title(ctx, id, false)
}

func (s *Service) Title(ctx context.Context, id uuid.UUID) (Title, bool, error) {
	return s.title(ctx, id, true)
}

func (s *Service) title(ctx context.Context, id uuid.UUID, complete bool) (Title, bool, error) {
	kind, tmdbID, ok := Parse(id)
	if !ok || !s.provider.Enabled() {
		return Title{}, false, nil
	}

	s.mutex.Lock()
	known, cached := s.details[id]
	s.mutex.Unlock()
	if cached && (known.complete || !complete) && s.now().Sub(known.fetched) < detailFor {
		return known.title, true, nil
	}

	lookup := s.provider.Movie
	if kind == itemmodel.KindSeries {
		lookup = s.provider.Series
	}

	metadata, found, err := lookup(ctx, int(tmdbID))
	if err != nil {
		if cached {
			return known.title, true, nil
		}

		return Title{}, false, fmt.Errorf("failed to read %s: %w", Key(kind, tmdbID), err)
	}
	if !found {
		return Title{}, false, nil
	}

	title := Title{ID: id, Kind: kind, TmdbID: tmdbID, Key: Key(kind, tmdbID), Metadata: metadata}
	s.mutex.Lock()
	s.remember(title, true)
	s.mutex.Unlock()

	return title, true, nil
}

func (t Title) Image(kind items.ImageKind) (string, bool) {
	for _, image := range t.Metadata.Images {
		if image.Kind == kind {
			return image.URL, true
		}
	}

	return "", false
}

func Tag(url string) string {
	sum := sha1.Sum([]byte(url))

	return hex.EncodeToString(sum[:])
}
