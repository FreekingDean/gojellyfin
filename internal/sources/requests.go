package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	"github.com/FreekingDean/gojellyfin/internal/sources/radarr"
	"github.com/FreekingDean/gojellyfin/internal/sources/sonarr"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type Wanted struct {
	Name   string
	TmdbID int
	TvdbID int
}

type record = map[string]any

func (s *Service) Requestable(ctx context.Context, kind Kind) ([]Source, error) {
	records, err := s.store.Source.Query().
		Where(sourcemodel.KindEQ(kind), sourcemodel.RequestProfileIDNotNil()).
		Order(sourcemodel.ByName()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the sources that take requests: %w", err)
	}

	requestable := make([]Source, len(records))
	for i, found := range records {
		requestable[i] = *found
	}

	return requestable, nil
}

func (s *Service) Submit(ctx context.Context, source Source, wanted Wanted) error {
	if source.RequestProfileID == nil {
		return fmt.Errorf("%s takes no requests", source.Name)
	}

	found, path, err := s.lookup(ctx, source, wanted)
	if err != nil {
		return err
	}
	if id, _ := found["id"].(float64); id > 0 {
		return nil
	}

	tags, err := s.requestTags(ctx, source)
	if err != nil {
		return err
	}

	found["qualityProfileId"] = *source.RequestProfileID
	found["rootFolderPath"] = source.RequestRootFolder
	found["monitored"] = true
	found["tags"] = tags
	if source.Kind == KindSonarr {
		found["seasonFolder"] = true
		found["addOptions"] = record{"monitor": "all", "searchForMissingEpisodes": true}
	} else {
		found["minimumAvailability"] = "released"
		found["addOptions"] = record{"searchForMovie": true}
	}

	return s.post(ctx, source, path, found)
}

func (s *Service) lookup(ctx context.Context, source Source, wanted Wanted) (record, string, error) {
	if source.Kind == KindSonarr {
		if wanted.TvdbID <= 0 {
			return nil, "", fmt.Errorf("%s has no TVDB id, which is what %s adds a series by", wanted.Name, source.Name)
		}

		matches, err := fetch[[]record](ctx, s, s.client, source, sonarr.LookupPath, url.Values{"term": {"tvdb:" + strconv.Itoa(wanted.TvdbID)}})
		if err != nil {
			return nil, "", err
		}
		if len(matches) == 0 {
			return nil, "", fmt.Errorf("%s does not know %s", source.Name, wanted.Name)
		}

		return matches[0], sonarr.SeriesPath, nil
	}

	found, err := fetch[record](ctx, s, s.client, source, radarr.LookupPath, url.Values{"tmdbId": {strconv.Itoa(wanted.TmdbID)}})
	if err != nil {
		return nil, "", err
	}

	return found, radarr.MoviesPath, nil
}

func (s *Service) requestTags(ctx context.Context, source Source) ([]int, error) {
	if source.RequestTag == "" {
		return []int{}, nil
	}

	tags, err := fetch[[]arr.Tag](ctx, s, s.client, source, arr.TagsPath, nil)
	if err != nil {
		return nil, err
	}
	for _, tag := range tags {
		if strings.EqualFold(tag.Label, source.RequestTag) {
			return []int{tag.ID}, nil
		}
	}

	return nil, fmt.Errorf("%s has no tag %q", source.Name, source.RequestTag)
}

func (s *Service) post(ctx context.Context, source Source, path string, body record) error {
	return s.send(ctx, source, http.MethodPost, path, body)
}

func (s *Service) send(ctx context.Context, source Source, method, path string, body any) error {
	parsed, err := url.Parse(source.URL)
	if err != nil {
		return err
	}

	var payload io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, parsed.JoinPath(path).String(), payload)
	if err != nil {
		return err
	}
	apiKey, err := s.APIKey(source.APIKeyVariable)
	if err != nil {
		return err
	}
	req.Header.Set(arr.APIKeyName, apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s answered %s for %s %s", parsed.Host, resp.Status, method, path)
	}

	return nil
}
