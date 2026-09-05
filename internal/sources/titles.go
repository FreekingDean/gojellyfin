package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	"github.com/FreekingDean/gojellyfin/internal/sources/radarr"
	"github.com/FreekingDean/gojellyfin/internal/sources/sonarr"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type File struct {
	Path         string
	DateModified time.Time
}

type Title struct {
	Tagged      bool
	Kind        itemmodel.Kind
	Name        string
	Year        *int32
	Index       *int32
	ParentIndex *int32
	Files       []File
	Children    []Title
}

type Binding struct {
	Source  Source
	Library Library
}

func (s *Service) Titles(ctx context.Context, binding Binding) ([]Title, error) {
	switch binding.Source.Kind {
	case KindRadarr:
		return s.movies(ctx, binding)
	case KindSonarr:
		return s.series(ctx, binding)
	default:
		return nil, fmt.Errorf("unsupported source kind %q", binding.Source.Kind)
	}
}

func (s *Service) movies(ctx context.Context, binding Binding) ([]Title, error) {
	tagged, err := s.tagFilter(ctx, binding)
	if err != nil {
		return nil, err
	}

	movies, err := get[[]radarr.Movie](ctx, s, binding.Source, radarr.MoviesPath, nil)
	if err != nil {
		return nil, err
	}

	titles := make([]Title, 0, len(movies))
	for _, movie := range movies {
		if !movie.HasFile {
			continue
		}

		found, err := file(binding.Source, movie.File)
		if err != nil {
			return nil, err
		}

		titles = append(titles, Title{
			Tagged: tagged(movie.Tags),
			Kind:   itemmodel.KindMovie,
			Name:   movie.Title,
			Year:   released(movie.Year),
			Files:  []File{found},
		})
	}

	return titles, nil
}

func (s *Service) series(ctx context.Context, binding Binding) ([]Title, error) {
	tagged, err := s.tagFilter(ctx, binding)
	if err != nil {
		return nil, err
	}

	shows, err := get[[]sonarr.Series](ctx, s, binding.Source, sonarr.SeriesPath, nil)
	if err != nil {
		return nil, err
	}

	titles := make([]Title, 0, len(shows))
	for _, show := range shows {
		query := url.Values{}
		query.Set("seriesId", strconv.Itoa(show.ID))
		query.Set("includeEpisodeFile", "true")

		jobs.Heartbeat(ctx, show.Title)

		episodes, err := get[[]sonarr.Episode](ctx, s, binding.Source, sonarr.EpisodesPath, query)
		if err != nil {
			return nil, err
		}

		children := seasons(binding.Source, episodes)
		if len(children) == 0 {
			continue
		}

		titles = append(titles, Title{
			Tagged:   tagged(show.Tags),
			Kind:     itemmodel.KindSeries,
			Name:     show.Title,
			Year:     released(show.Year),
			Children: children,
		})
	}

	return titles, nil
}

func seasons(source Source, episodes []sonarr.Episode) []Title {
	numbers := make([]int32, 0)
	byNumber := map[int32][]Title{}

	for _, episode := range episodes {
		if !episode.HasFile {
			continue
		}

		found, err := file(source, episode.File)
		if err != nil {
			log.Printf("skipping %s: %v", episode.Title, err)

			continue
		}

		if _, ok := byNumber[episode.SeasonNumber]; !ok {
			numbers = append(numbers, episode.SeasonNumber)
		}

		byNumber[episode.SeasonNumber] = append(byNumber[episode.SeasonNumber], Title{
			Kind:        itemmodel.KindEpisode,
			Name:        episode.Title,
			Index:       ptr(episode.EpisodeNumber),
			ParentIndex: ptr(episode.SeasonNumber),
			Files:       []File{found},
		})
	}

	slices.Sort(numbers)

	titles := make([]Title, 0, len(numbers))
	for _, number := range numbers {
		titles = append(titles, Title{
			Kind:     itemmodel.KindSeason,
			Index:    ptr(number),
			Children: byNumber[number],
		})
	}

	return titles
}

func (s *Service) tagFilter(ctx context.Context, binding Binding) (func([]int) bool, error) {
	if binding.Library.TagFilter == "" {
		return func([]int) bool { return true }, nil
	}

	tags, err := get[[]arr.Tag](ctx, s, binding.Source, arr.TagsPath, nil)
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		if strings.EqualFold(tag.Label, binding.Library.TagFilter) {
			return func(ids []int) bool { return slices.Contains(ids, tag.ID) }, nil
		}
	}

	return nil, fmt.Errorf("%s has no tag %q", binding.Source.Name, binding.Library.TagFilter)
}

func file(source Source, from arr.File) (File, error) {
	path, err := Localise(source, from.Path)
	if err != nil {
		return File{}, err
	}

	return File{Path: path, DateModified: from.DateAdded}, nil
}

func Localise(source Source, path string) (string, error) {
	root := strings.TrimSuffix(source.RootPath, string(filepath.Separator))
	if root == "" {
		return "", fmt.Errorf("%s names no root path", source.Name)
	}

	if path != root && !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return "", fmt.Errorf("%s reported %q, which is outside its root %s", source.Name, path, root)
	}

	return filepath.Join(source.LocalPath, strings.TrimPrefix(path, root)), nil
}

func released(year int32) *int32 {
	if year == 0 {
		return nil
	}

	return &year
}

func ptr[T any](value T) *T {
	return &value
}

func get[T any](ctx context.Context, s *Service, source Source, path string, query url.Values) (T, error) {
	var out T

	parsed, err := url.Parse(source.URL)
	if err != nil {
		return out, err
	}

	target := parsed.JoinPath(path)
	target.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return out, err
	}
	apiKey, err := s.APIKey(source.APIKeyVariable)
	if err != nil {
		return out, err
	}
	req.Header.Set(arr.APIKeyName, apiKey)

	resp, err := s.lister.Do(req)
	if err != nil {
		return out, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("%s answered %s for %s", parsed.Host, resp.Status, path)
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}

	return out, nil
}
