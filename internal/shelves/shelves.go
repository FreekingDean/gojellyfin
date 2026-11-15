package shelves

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
)

var namespace = uuid.MustParse("6f1d7c2e-9a43-4f5b-8c1e-3d2a7b9e0c41")

type Catalogue interface {
	LatestItems(ctx context.Context, query items.LatestQuery) ([]items.Latest, error)
	DistinctGenres(ctx context.Context, query items.MetadataQuery) ([]items.Named, int, error)
	LastPlayed(ctx context.Context, viewer items.Viewer, userID uuid.UUID, kind items.Kind) (*items.Item, []string, error)
	Posters(ctx context.Context, records []*items.Item, limit int) ([]*items.Image, error)
}

type Discovery interface {
	Enabled() bool
	Trending(ctx context.Context, kind items.Kind, limit int) ([]discover.Title, error)
}

type Shelf struct {
	ID         uuid.UUID
	Slug       string
	Name       string
	Collection libraries.CollectionType
	Unowned    items.Kind
	query      items.LatestQuery
}

type Card struct {
	Name     string
	Fresh    int
	Posters  []*items.Image
	Personal bool
}

const (
	FreshLimit  = 99
	FreshWindow = 7 * 24 * time.Hour
)

type Settings interface {
	Configuration(ctx context.Context, key string) (json.RawMessage, error)
	SetConfiguration(ctx context.Context, key string, value json.RawMessage) error
}

type Service struct {
	catalogue Catalogue
	discovery Discovery
	settings  Settings
	now       func() time.Time
}

type Choice struct {
	Slug    string
	Name    string
	Enabled bool
}

const settingsKey = "shelves"

var ErrUnknown = errors.New("no such shelf")

func New(catalogue *items.Service, discovery *discover.Service, settings *config.Service) *Service {
	return newService(catalogue, discovery, settings, time.Now)
}

func newService(catalogue Catalogue, discovery Discovery, settings Settings, now func() time.Time) *Service {
	return &Service{catalogue: catalogue, discovery: discovery, settings: settings, now: now}
}

func (s *Service) slugs(ctx context.Context) ([]string, error) {
	stored, err := s.settings.Configuration(ctx, settingsKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read which shelves are on: %w", err)
	}
	if stored == nil {
		return Defaults, nil
	}

	var slugs []string
	if err := json.Unmarshal(stored, &slugs); err != nil {
		return nil, fmt.Errorf("failed to read which shelves are on: %w", err)
	}

	return slugs, nil
}

func (s *Service) enabled(ctx context.Context) ([]definition, error) {
	slugs, err := s.slugs(ctx)
	if err != nil {
		return nil, err
	}

	enabled := make([]definition, 0, len(slugs))
	for _, entry := range definitions {
		if slices.Contains(slugs, entry.slug) {
			enabled = append(enabled, entry)
		}
	}

	return enabled, nil
}

func (s *Service) Choices(ctx context.Context) ([]Choice, error) {
	slugs, err := s.slugs(ctx)
	if err != nil {
		return nil, err
	}

	choices := make([]Choice, 0, len(definitions))
	for _, entry := range definitions {
		choices = append(choices, Choice{Slug: entry.slug, Name: entry.name(), Enabled: slices.Contains(slugs, entry.slug)})
	}

	return choices, nil
}

func (s *Service) Enable(ctx context.Context, slugs []string) error {
	known := Slugs()
	enabled := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		if !slices.Contains(known, slug) {
			return fmt.Errorf("%w: %q is not one of %s", ErrUnknown, slug, strings.Join(known, ", "))
		}
		if !slices.Contains(enabled, slug) {
			enabled = append(enabled, slug)
		}
	}

	value, err := json.Marshal(enabled)
	if err != nil {
		return err
	}

	return s.settings.SetConfiguration(ctx, settingsKey, value)
}

func (d definition) name() string {
	if d.title != "" {
		return d.title
	}

	words := strings.Split(d.slug, "-")
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}

	return strings.Join(words, " ")
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

type Title struct {
	ID   uuid.UUID
	Name string
}

func (s *Service) Titles(ctx context.Context) ([]Title, error) {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}

	titles := make([]Title, 0, len(enabled))
	for _, entry := range enabled {
		if entry.title != "" {
			titles = append(titles, Title{ID: ID(entry.slug), Name: entry.title})
		}
	}

	return titles, nil
}

func (s *Service) Shelves(ctx context.Context, viewer items.Viewer, userID uuid.UUID) ([]Shelf, error) {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}

	shelves := make([]Shelf, 0, len(enabled))
	for _, entry := range enabled {
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
	enabled, err := s.enabled(ctx)
	if err != nil {
		return Shelf{}, false, err
	}

	for _, entry := range enabled {
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

	if entry.unowned != "" && !s.discovery.Enabled() {
		ok = false
	}

	query.Viewer = viewer
	return Shelf{ID: ID(entry.slug), Slug: entry.slug, Name: name, Collection: entry.collection, Unowned: entry.unowned, query: query}, ok, nil
}

func (s *Service) Latest(ctx context.Context, shelf Shelf, limit int, ungrouped bool) ([]items.Latest, error) {
	query := shelf.query
	query.Now = s.now()
	query.Limit = limit
	query.Ungrouped = ungrouped

	return s.catalogue.LatestItems(ctx, query)
}

func (s *Service) Card(ctx context.Context, id uuid.UUID, viewer items.Viewer, userID uuid.UUID, limit int) (Card, bool, error) {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return Card{}, false, err
	}

	index := slices.IndexFunc(enabled, func(entry definition) bool { return ID(entry.slug) == id })
	if index < 0 {
		return Card{}, false, nil
	}

	entry := enabled[index]
	if entry.unowned != "" {
		return s.trendingCard(ctx, entry, limit)
	}

	if entry.shared {
		userID = uuid.Nil
	} else if userID == uuid.Nil {
		return Card{}, false, nil
	}

	shelf, ok, err := s.resolve(ctx, entry, viewer, userID)
	if err != nil || !ok {
		return Card{}, false, err
	}

	latest, err := s.Latest(ctx, shelf, limit*2, false)
	if err != nil {
		return Card{}, false, err
	}

	records := make([]*items.Item, 0, len(latest))
	for _, entry := range latest {
		records = append(records, entry.Item)
	}

	posters, err := s.catalogue.Posters(ctx, records, limit)
	if err != nil {
		return Card{}, false, err
	}

	fresh, err := s.fresh(ctx, shelf)

	return Card{Name: shelf.Name, Fresh: fresh, Posters: posters, Personal: !entry.shared}, true, err
}

func (s *Service) trendingCard(ctx context.Context, entry definition, limit int) (Card, bool, error) {
	if !s.discovery.Enabled() {
		return Card{}, false, nil
	}

	titles, err := s.discovery.Trending(ctx, entry.unowned, limit)
	if err != nil {
		return Card{}, false, err
	}

	posters := make([]*items.Image, 0, len(titles))
	for _, title := range titles {
		if url, ok := title.Image(imagemodel.KindPrimary); ok {
			posters = append(posters, &items.Image{Kind: imagemodel.KindPrimary, URL: url, Tag: url})
		}
	}

	return Card{Name: entry.title, Posters: posters}, true, nil
}

func (s *Service) fresh(ctx context.Context, shelf Shelf) (int, error) {
	aired, err := s.Latest(ctx, shelf, FreshLimit, true)
	if err != nil {
		return 0, err
	}

	now := s.now()
	since := now.Add(-FreshWindow)
	count := 0
	for _, entry := range aired {
		if premiere := entry.Item.PremiereDate; premiere != nil && premiere.After(since) && !premiere.After(now) {
			count++
		}
	}

	return count, nil
}
