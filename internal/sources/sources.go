package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/entities"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

const (
	testTimeout = 10 * time.Second
	listTimeout = 2 * time.Minute

	KindRadarr = sourcemodel.KindRadarr
	KindSonarr = sourcemodel.KindSonarr
)

type Service struct {
	client *http.Client
	lister *http.Client
	store  *store.Client
}

func New(store *store.Client) *Service {
	return &Service{
		client: &http.Client{Timeout: testTimeout},
		lister: &http.Client{Timeout: listTimeout},
		store:  store,
	}
}

type (
	Kind        = sourcemodel.Kind
	Source      = store.Source
	PathMapping = entities.SourcePathMapping
	Library     = entities.SourceLibrary
)

func (s *Service) List(ctx context.Context) ([]Source, error) {
	sourcePtr, err := s.store.Source.Query().
		Order(sourcemodel.ByName()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	sources := make([]Source, len(sourcePtr))
	for i, source := range sourcePtr {
		sources[i] = *source
	}
	return sources, nil
}

func (s *Service) Update(ctx context.Context, sources []Source) error {
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		namesToUpdate := make([]string, len(sources))
		for i, source := range sources {
			namesToUpdate[i] = source.Name
		}
		_, err := tx.Source.Delete().
			Where(sourcemodel.NameNotIn(namesToUpdate...)).
			Exec(ctx)
		if err != nil {
			return err
		}

		for _, source := range sources {
			err := tx.Source.Create().
				SetName(source.Name).
				SetURL(source.URL).
				SetAPIKey(source.APIKey).
				SetPathMappings(source.PathMappings).
				SetLibraries(source.Libraries).
				SetKind(source.Kind).
				OnConflictColumns(sourcemodel.FieldName).
				UpdateNewValues().
				Exec(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (s *Service) Test(ctx context.Context, apiURL, apiKey string) error {
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("no host in %q", apiURL)
	}

	target := parsed.JoinPath(arr.StatusPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set(arr.APIKeyName, apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %s", parsed.Host, resp.Status)
	}

	return nil
}
