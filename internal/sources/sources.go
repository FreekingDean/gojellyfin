package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	"github.com/FreekingDean/gojellyfin/internal/store"
	librarysourcemodel "github.com/FreekingDean/gojellyfin/internal/store/librarysource"
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
	config env.Config
}

func New(store *store.Client, config env.Config) *Service {
	return &Service{
		client: &http.Client{Timeout: testTimeout},
		lister: &http.Client{Timeout: listTimeout},
		store:  store,
		config: config,
	}
}

func (s *Service) APIKey(variable string) (string, error) {
	if !env.ValidSourceAPIKeyVariable(variable) {
		return "", fmt.Errorf("%q is not an API key variable: the name must begin with %s", variable, env.SourceAPIKeyPrefix)
	}

	key := s.config.SourceAPIKeys[variable]
	if key == "" {
		return "", fmt.Errorf("%s is not set on this server", variable)
	}

	return key, nil
}

type (
	Kind   = sourcemodel.Kind
	Source = store.Source
)

type Library struct {
	ID         uuid.UUID
	TagFilter  string
	SourcePath string
	TargetPath string
}

type Configured struct {
	Source    Source
	Libraries []Library
}

func (s *Service) List(ctx context.Context) ([]Configured, error) {
	records, err := s.store.Source.Query().
		WithLibraries().
		Order(sourcemodel.ByName()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	configured := make([]Configured, len(records))
	for i, record := range records {
		configured[i] = Configured{
			Source:    *record,
			Libraries: libraries(record.Edges.Libraries),
		}
	}

	return configured, nil
}

func (s *Service) BindingsFor(ctx context.Context, id uuid.UUID) ([]Binding, error) {
	records, err := s.store.LibrarySource.Query().
		Where(librarysourcemodel.LibraryID(id)).
		WithSource().
		All(ctx)
	if err != nil {
		return nil, err
	}

	bindings := make([]Binding, 0, len(records))
	for _, record := range records {
		if record.Edges.Source == nil {
			continue
		}

		bindings = append(bindings, Binding{
			Source:  *record.Edges.Source,
			Library: library(record),
		})
	}

	slices.SortFunc(bindings, func(first, second Binding) int {
		return strings.Compare(first.Source.Name, second.Source.Name)
	})

	return bindings, nil
}

func libraries(records []*store.LibrarySource) []Library {
	bound := make([]Library, len(records))
	for i, record := range records {
		bound[i] = library(record)
	}

	return bound
}

func library(record *store.LibrarySource) Library {
	return Library{
		ID:         record.LibraryID,
		TagFilter:  record.TagFilter,
		SourcePath: record.SourcePath,
		TargetPath: record.TargetPath,
	}
}

func (s *Service) Update(ctx context.Context, configured []Configured) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		names := make([]string, len(configured))
		for i, entry := range configured {
			names[i] = entry.Source.Name
		}

		if _, err := tx.Source.Delete().
			Where(sourcemodel.NameNotIn(names...)).
			Exec(ctx); err != nil {
			return err
		}

		for _, entry := range configured {
			if !env.ValidSourceAPIKeyVariable(entry.Source.APIKeyVariable) {
				return fmt.Errorf(
					"%s names %q as its API key variable, which must begin with %s",
					entry.Source.Name, entry.Source.APIKeyVariable, env.SourceAPIKeyPrefix,
				)
			}

			id, err := tx.Source.Create().
				SetName(entry.Source.Name).
				SetURL(entry.Source.URL).
				SetAPIKeyVariable(entry.Source.APIKeyVariable).
				SetKind(entry.Source.Kind).
				OnConflictColumns(sourcemodel.FieldName).
				UpdateNewValues().
				ID(ctx)
			if err != nil {
				return err
			}

			if _, err := tx.LibrarySource.Delete().
				Where(librarysourcemodel.SourceID(id)).
				Exec(ctx); err != nil {
				return err
			}

			for _, bound := range entry.Libraries {
				if err := tx.LibrarySource.Create().
					SetSourceID(id).
					SetLibraryID(bound.ID).
					SetTagFilter(bound.TagFilter).
					SetSourcePath(bound.SourcePath).
					SetTargetPath(bound.TargetPath).
					Exec(ctx); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (s *Service) Test(ctx context.Context, apiURL, variable string) error {
	apiKey, err := s.APIKey(variable)
	if err != nil {
		return err
	}

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
