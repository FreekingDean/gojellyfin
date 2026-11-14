package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

type instance struct {
	server *httptest.Server

	mutex  sync.Mutex
	asked  []string
	posted []map[string]any
}

func (i *instance) requests() []string {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	return slices.Clone(i.asked)
}

func (i *instance) bodies() []map[string]any {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	return slices.Clone(i.posted)
}

func newInstance(t *testing.T, answers map[string]any) *instance {
	t.Helper()

	started := &instance{}
	started.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" {
			asked += "?" + r.URL.RawQuery
		}

		started.mutex.Lock()
		started.asked = append(started.asked, asked)
		if r.Method == http.MethodPost {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			started.posted = append(started.posted, body)
		}
		started.mutex.Unlock()

		if r.Header.Get("X-Api-Key") != "key" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		answer, known := answers[r.Method+" "+r.URL.Path]
		if !known {
			w.WriteHeader(http.StatusNotFound)

			return
		}
		if status, refused := answer.(int); refused {
			w.WriteHeader(status)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_ = json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(started.server.Close)

	return started
}

func profile(id int) *int { return &id }

func submitter() *Service {
	return &Service{
		client: &http.Client{Timeout: testTimeout},
		config: env.Config{SourceAPIKeys: map[string]string{"SOURCE_API_KEY_TEST": "key"}},
	}
}

func (i *instance) source(kind Kind, tag string) Source {
	return Source{
		Name:              "instance",
		URL:               i.server.URL,
		APIKeyVariable:    "SOURCE_API_KEY_TEST",
		Kind:              kind,
		RequestProfileID:  profile(4),
		RequestRootFolder: "/movies",
		RequestTag:        tag,
	}
}

var tags = []map[string]any{{"id": 1, "label": "kids"}, {"id": 2, "label": "Requested"}}

func TestSubmit_Radarr(t *testing.T) {
	ctx := context.Background()
	wanted := Wanted{Name: "The Matrix", TmdbID: 603}

	t.Run("adds what the instance looked up, where the source says requests go", func(t *testing.T) {
		radarr := newInstance(t, map[string]any{
			"GET /api/v3/movie/lookup/tmdb": map[string]any{"tmdbId": 603, "title": "The Matrix", "year": 1999, "titleSlug": "the-matrix-603"},
			"GET /api/v3/tag":               tags,
			"POST /api/v3/movie":            map[string]any{"id": 12},
		})

		if err := submitter().Submit(ctx, radarr.source(KindRadarr, "requested"), wanted); err != nil {
			t.Fatalf("failed to submit: %v", err)
		}

		want := []string{"GET /api/v3/movie/lookup/tmdb?tmdbId=603", "GET /api/v3/tag", "POST /api/v3/movie"}
		if got := radarr.requests(); !slices.Equal(got, want) {
			t.Fatalf("asked = %v, want %v", got, want)
		}

		body := radarr.bodies()[0]
		for field, value := range map[string]any{
			"titleSlug":           "the-matrix-603",
			"tmdbId":              float64(603),
			"qualityProfileId":    float64(4),
			"rootFolderPath":      "/movies",
			"monitored":           true,
			"minimumAvailability": "released",
		} {
			if body[field] != value {
				t.Errorf("%s = %v, want %v", field, body[field], value)
			}
		}
		if got := body["tags"].([]any); len(got) != 1 || got[0] != float64(2) {
			t.Errorf("tags = %v, want the one the source names", got)
		}
		if got := body["addOptions"].(map[string]any)["searchForMovie"]; got != true {
			t.Errorf("searchForMovie = %v, want a search started", got)
		}
	})

	t.Run("a source naming no tag asks for none and sends none", func(t *testing.T) {
		radarr := newInstance(t, map[string]any{
			"GET /api/v3/movie/lookup/tmdb": map[string]any{"tmdbId": 603},
			"POST /api/v3/movie":            map[string]any{"id": 12},
		})

		if err := submitter().Submit(ctx, radarr.source(KindRadarr, ""), wanted); err != nil {
			t.Fatalf("failed to submit: %v", err)
		}
		if got := radarr.bodies()[0]["tags"].([]any); len(got) != 0 {
			t.Errorf("tags = %v, want none", got)
		}
		if slices.Contains(radarr.requests(), "GET /api/v3/tag") {
			t.Error("the tags were read for a source that names none")
		}
	})

	t.Run("a title the instance already manages is not added again", func(t *testing.T) {
		radarr := newInstance(t, map[string]any{
			"GET /api/v3/movie/lookup/tmdb": map[string]any{"id": 12, "tmdbId": 603},
		})

		if err := submitter().Submit(ctx, radarr.source(KindRadarr, ""), wanted); err != nil {
			t.Fatalf("failed to submit: %v", err)
		}
		if posted := radarr.bodies(); len(posted) != 0 {
			t.Errorf("posted = %v, want nothing", posted)
		}
	})

	failures := []struct {
		name    string
		answers map[string]any
		source  func(*instance) Source
		want    string
	}{
		{
			name:    "a tag the instance does not define",
			answers: map[string]any{"GET /api/v3/movie/lookup/tmdb": map[string]any{"tmdbId": 603}, "GET /api/v3/tag": tags},
			source:  func(i *instance) Source { return i.source(KindRadarr, "4k") },
			want:    `has no tag "4k"`,
		},
		{
			name:    "a title the instance cannot look up",
			answers: map[string]any{},
			source:  func(i *instance) Source { return i.source(KindRadarr, "") },
			want:    "404",
		},
		{
			name: "an add the instance refuses",
			answers: map[string]any{
				"GET /api/v3/movie/lookup/tmdb": map[string]any{"tmdbId": 603},
				"POST /api/v3/movie":            http.StatusBadRequest,
			},
			source: func(i *instance) Source { return i.source(KindRadarr, "") },
			want:   "400",
		},
		{
			name:    "a source that takes no requests",
			answers: map[string]any{},
			source: func(i *instance) Source {
				source := i.source(KindRadarr, "")
				source.RequestProfileID = nil

				return source
			},
			want: "takes no requests",
		},
	}

	for _, test := range failures {
		t.Run(test.name, func(t *testing.T) {
			radarr := newInstance(t, test.answers)

			err := submitter().Submit(ctx, test.source(radarr), wanted)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("err = %v, want one naming %q", err, test.want)
			}
			if posted := radarr.bodies(); len(posted) > 0 && test.want != "400" {
				t.Errorf("posted = %v, want nothing", posted)
			}
		})
	}
}

func TestSubmit_Sonarr(t *testing.T) {
	ctx := context.Background()

	t.Run("looks the series up by its TVDB id and monitors all of it", func(t *testing.T) {
		sonarr := newInstance(t, map[string]any{
			"GET /api/v3/series/lookup": []map[string]any{{"tvdbId": 81189, "title": "Breaking Bad", "seasons": []map[string]any{{"seasonNumber": 1}}}},
			"POST /api/v3/series":       map[string]any{"id": 3},
		})

		err := submitter().Submit(ctx, sonarr.source(KindSonarr, ""), Wanted{Name: "Breaking Bad", TmdbID: 1396, TvdbID: 81189})
		if err != nil {
			t.Fatalf("failed to submit: %v", err)
		}

		want := []string{"GET /api/v3/series/lookup?term=tvdb%3A81189", "POST /api/v3/series"}
		if got := sonarr.requests(); !slices.Equal(got, want) {
			t.Fatalf("asked = %v, want %v", got, want)
		}

		body := sonarr.bodies()[0]
		if body["tvdbId"] != float64(81189) || body["seasonFolder"] != true || body["qualityProfileId"] != float64(4) {
			t.Errorf("posted = %v", body)
		}
		options := body["addOptions"].(map[string]any)
		if options["monitor"] != "all" || options["searchForMissingEpisodes"] != true {
			t.Errorf("addOptions = %v", options)
		}
		if _, minimum := body["minimumAvailability"]; minimum {
			t.Error("a series was given a movie's availability")
		}
	})

	t.Run("a series with no TVDB id is refused before a request", func(t *testing.T) {
		sonarr := newInstance(t, map[string]any{})

		err := submitter().Submit(ctx, sonarr.source(KindSonarr, ""), Wanted{Name: "Breaking Bad", TmdbID: 1396})
		if err == nil || !strings.Contains(err.Error(), "TVDB") {
			t.Errorf("err = %v, want one naming the TVDB id", err)
		}
		if asked := sonarr.requests(); len(asked) != 0 {
			t.Errorf("asked = %v, want nothing", asked)
		}
	})

	t.Run("a series the instance cannot find is refused", func(t *testing.T) {
		sonarr := newInstance(t, map[string]any{"GET /api/v3/series/lookup": []map[string]any{}})

		err := submitter().Submit(ctx, sonarr.source(KindSonarr, ""), Wanted{Name: "Breaking Bad", TmdbID: 1396, TvdbID: 81189})
		if err == nil || !strings.Contains(err.Error(), "does not know") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestRequestable(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t, &[]movie{})

	listed := func(kind Kind) bool {
		t.Helper()

		requestable, err := fixture.service.Requestable(ctx, kind)
		if err != nil {
			t.Fatalf("failed to list the sources: %v", err)
		}

		return slices.ContainsFunc(requestable, func(source Source) bool { return source.ID == fixture.sourceID })
	}

	if listed(KindRadarr) {
		t.Error("a source with no quality profile takes requests")
	}

	if err := fixture.client.Source.UpdateOneID(fixture.sourceID).SetRequestProfileID(4).SetRequestRootFolder("/media").Exec(ctx); err != nil {
		t.Fatalf("failed to configure the source: %v", err)
	}

	if !listed(KindRadarr) {
		t.Error("a source with a quality profile takes no requests")
	}
	if listed(KindSonarr) {
		t.Error("a Radarr was offered a series")
	}
}

func TestUpdate_RequestSettings(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t, &[]movie{})
	fixture.library(t, "")

	save := func(change func(*Source)) error {
		entry := fixture.configured(t)
		change(&entry.Source)
		_, err := fixture.service.Update(ctx, fixture.sourceID, entry)

		return err
	}

	refused := map[string]func(*Source){
		"a folder the source does not map": func(source *Source) {
			source.RequestProfileID = profile(4)
			source.RequestRootFolder = "/elsewhere"
		},
		"a profile and no folder": func(source *Source) {
			source.RequestProfileID = profile(4)
		},
		"a profile no instance defines": func(source *Source) {
			source.RequestProfileID = profile(0)
			source.RequestRootFolder = "/media"
		},
		"a folder and no profile": func(source *Source) {
			source.RequestRootFolder = "/media"
		},
	}
	for name, change := range refused {
		if err := save(change); err == nil {
			t.Errorf("%s was saved", name)
		}
	}

	if err := save(func(source *Source) {
		source.RequestProfileID = profile(4)
		source.RequestRootFolder = "/media"
		source.RequestTag = "requested"
	}); err != nil {
		t.Fatalf("failed to turn requests on: %v", err)
	}
	saved := fixture.configured(t).Source
	if saved.RequestProfileID == nil || *saved.RequestProfileID != 4 || saved.RequestRootFolder != "/media" || saved.RequestTag != "requested" {
		t.Errorf("saved = %+v", saved)
	}

	if err := save(func(source *Source) {
		source.RequestProfileID = nil
		source.RequestRootFolder = ""
		source.RequestTag = ""
	}); err != nil {
		t.Fatalf("failed to turn requests off: %v", err)
	}
	if saved := fixture.configured(t).Source; saved.RequestProfileID != nil {
		t.Errorf("profile = %d after requests were turned off", *saved.RequestProfileID)
	}
}
