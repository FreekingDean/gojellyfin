package shelves

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

var namespace = uuid.MustParse("6f1d7c2e-9a43-4f5b-8c1e-3d2a7b9e0c41")

type Catalogue interface {
	LatestItems(ctx context.Context, query items.LatestQuery) ([]items.Latest, error)
	DistinctGenres(ctx context.Context, query items.MetadataQuery) ([]items.Named, int, error)
	LastPlayed(ctx context.Context, viewer items.Viewer, userID uuid.UUID, kind items.Kind) (*items.Item, []string, error)
}

type Shelf struct {
	ID         uuid.UUID
	Slug       string
	Name       string
	Collection libraries.CollectionType
	query      items.LatestQuery
}

type Service struct {
	catalogue Catalogue
	enabled   []definition
	now       func() time.Time
}

func New(catalogue *items.Service, config env.Config) (*Service, error) {
	return newService(catalogue, config.Shelves, time.Now)
}

func newService(catalogue Catalogue, slugs []string, now func() time.Time) (*Service, error) {
	if slugs == nil {
		slugs = Defaults
	}

	enabled := make([]definition, 0, len(slugs))
	for _, slug := range slugs {
		if strings.EqualFold(slug, "none") {
			continue
		}
		index := slices.IndexFunc(definitions, func(candidate definition) bool { return candidate.slug == slug })
		if index < 0 {
			return nil, fmt.Errorf("SHELVES names %q, which is not one of %s", slug, strings.Join(Slugs(), ", "))
		}
		enabled = append(enabled, definitions[index])
	}

	return &Service{catalogue: catalogue, enabled: enabled, now: now}, nil
}

func Slugs() []string {
	slugs := make([]string, 0, len(definitions))
	for _, entry := range definitions {
		slugs = append(slugs, entry.slug)
	}

	return slugs
}

func ID(slug string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte(slug))
}

func (s *Service) Shelves(ctx context.Context, viewer items.Viewer, userID uuid.UUID) ([]Shelf, error) {
	shelves := make([]Shelf, 0, len(s.enabled))
	for _, entry := range s.enabled {
		shelf, ok, err := s.resolve(ctx, entry, viewer, userID)
		if err != nil {
			return nil, err
		}
		if ok {
			shelves = append(shelves, shelf)
		}
	}

	return shelves, nil
}

func (s *Service) Shelf(ctx context.Context, id uuid.UUID, viewer items.Viewer, userID uuid.UUID) (Shelf, bool, error) {
	for _, entry := range s.enabled {
		if ID(entry.slug) == id {
			return s.resolve(ctx, entry, viewer, userID)
		}
	}

	return Shelf{}, false, nil
}

func (s *Service) resolve(ctx context.Context, entry definition, viewer items.Viewer, userID uuid.UUID) (Shelf, bool, error) {
	name, query, ok, err := entry.resolve(ctx, s.catalogue, viewer, userID, s.now())
	if err != nil {
		return Shelf{}, false, fmt.Errorf("failed to resolve the %s shelf: %w", entry.slug, err)
	}

	query.Viewer = viewer
	return Shelf{ID: ID(entry.slug), Slug: entry.slug, Name: name, Collection: entry.collection, query: query}, ok, nil
}

func (s *Service) Latest(ctx context.Context, shelf Shelf, limit int, ungrouped bool) ([]items.Latest, error) {
	query := shelf.query
	query.Now = s.now()
	query.Limit = limit
	query.Ungrouped = ungrouped

	return s.catalogue.LatestItems(ctx, query)
}
