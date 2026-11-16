package sources

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	itemsourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type downloader struct {
	sync.Mutex
	reported      *[]movie
	notifications []map[string]any
	nextID        int
	methods       []string
}

func (i *downloader) serve(w http.ResponseWriter, r *http.Request) {
	i.Lock()
	defer i.Unlock()

	switch {
	case strings.HasPrefix(r.URL.Path, "/api/v3/movie/"):
		id, _ := strconv.ParseInt(path.Base(r.URL.Path), 10, 64)
		for _, held := range *i.reported {
			if held.ID == id {
				_ = json.NewEncoder(w).Encode(held)

				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	case r.URL.Path == "/api/v3/notification/schema":
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"implementation": "Email", "fields": []any{}},
			{
				"implementation":              "Webhook",
				"onGrab":                      false,
				"onDownload":                  false,
				"onUpgrade":                   false,
				"onRename":                    false,
				"onMovieDelete":               false,
				"onMovieFileDelete":           false,
				"onMovieFileDeleteForUpgrade": false,
				"onHealthIssue":               false,
				"supportsOnMovieFileDelete":   false,
				"supportsOnDownload":          true,
				"includeHealthWarnings":       false,
				"fields": []any{
					map[string]any{"name": "url"},
					map[string]any{"name": "method", "value": 1},
					map[string]any{"name": "username"},
					map[string]any{"name": "password"},
				},
			},
		})
	case r.URL.Path == "/api/v3/notification" && r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(i.notifications)
	case r.URL.Path == "/api/v3/notification" && r.Method == http.MethodPost:
		i.methods = append(i.methods, r.Method)
		var created map[string]any
		_ = json.NewDecoder(r.Body).Decode(&created)
		i.nextID++
		created["id"] = float64(i.nextID)
		i.notifications = append(i.notifications, created)
		_ = json.NewEncoder(w).Encode(created)
	case strings.HasPrefix(r.URL.Path, "/api/v3/notification/"):
		i.methods = append(i.methods, r.Method)
		id, _ := strconv.Atoi(path.Base(r.URL.Path))
		for n, held := range i.notifications {
			if int(held["id"].(float64)) != id {
				continue
			}
			if r.Method == http.MethodDelete {
				i.notifications = append(i.notifications[:n], i.notifications[n+1:]...)

				return
			}
			var replaced map[string]any
			_ = json.NewDecoder(r.Body).Decode(&replaced)
			i.notifications[n] = replaced
			_ = json.NewEncoder(w).Encode(replaced)

			return
		}
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (i *downloader) held() []map[string]any {
	i.Lock()
	defer i.Unlock()

	encoded, _ := json.Marshal(i.notifications)
	var copied []map[string]any
	_ = json.Unmarshal(encoded, &copied)

	return copied
}

func (f *fixture) published(t *testing.T) *Service {
	t.Helper()

	config := f.config
	config.PublishedServerURL = "https://jellyfin.example/"

	return New(f.client, config, f.items, activity.New(f.client))
}

func (f *fixture) hooked(t *testing.T, service *Service, hooked Hooked) []jobs.Params {
	t.Helper()

	enqueued, err := jobs.RunJob(t, service.RefreshTitleJob(), hooked.Params(f.sourceID)...)
	if err != nil {
		t.Fatalf("failed to refresh the title: %v", err)
	}

	return jobs.Enqueued(t, enqueued, jobs.RefreshItem)
}

func (f *fixture) item(t *testing.T, key string) *items.Item {
	t.Helper()

	record, err := f.client.Item.Query().Where(itemmodel.Key(key)).OnlyModel(context.Background())
	if err != nil {
		t.Fatalf("failed to read %s back: %v", key, err)
	}

	return record
}

func (f *fixture) paths(t *testing.T) []string {
	t.Helper()

	held, err := f.client.ItemSource.Query().
		Where(itemsourcemodel.SourceID(f.sourceID)).
		Select(itemsourcemodel.FieldPath).
		Strings(context.Background())
	if err != nil {
		t.Fatalf("failed to read the files back: %v", err)
	}

	return held
}

func TestRefreshSourceTitle(t *testing.T) {
	matrix, heat := int(uuid.New().ID()), int(uuid.New().ID())
	reported := []movie{
		{ID: 41, TmdbID: matrix, Title: "The Matrix", Year: 1999, HasFile: true,
			Path: "/media/Matrix", File: movieFile{ID: 4100, Path: "/media/Matrix/" + t.Name() + ".mkv"}},
		{ID: 42, TmdbID: heat, Title: "Heat", Year: 1995, HasFile: true,
			Path: "/media/Heat", File: movieFile{ID: 4200, Path: "/media/Heat/" + t.Name() + ".mkv"}},
	}

	fixture := newFixture(t, &reported)
	library := fixture.library(t, "")
	fixture.write(t, library, fixture.refresh(t, library))
	t.Cleanup(func() {
		_, _ = fixture.client.Item.Delete().
			Where(itemmodel.KeyIn(items.MovieKey(matrix), items.MovieKey(heat))).
			Exec(context.Background())
	})

	t.Run("a rename reads that movie alone and keeps it listed", func(t *testing.T) {
		renamed := "/media/Matrix/" + t.Name() + ".mkv"
		fixture.downloader.Lock()
		reported[0].File = movieFile{ID: 4101, Path: renamed}
		fixture.downloader.Unlock()

		enqueued := fixture.hooked(t, fixture.service, Hooked{ID: 41})
		if len(enqueued) != 1 {
			t.Fatalf("enqueued = %d, want only the renamed movie", len(enqueued))
		}
		if fixture.item(t, items.MovieKey(matrix)).DeletedAt != nil {
			t.Error("the renamed movie was swept before its new file was written")
		}

		fixture.write(t, library, enqueued)

		held := fixture.paths(t)
		if !strings.Contains(strings.Join(held, ","), renamed) || len(held) != 2 {
			t.Errorf("files = %v, want the new path beside Heat's untouched one", held)
		}
	})

	t.Run("a delete drops the movie's file and sweeps it at once", func(t *testing.T) {
		fixture.downloader.Lock()
		reported = reported[:1]
		fixture.downloader.Unlock()

		enqueued := fixture.hooked(t, fixture.service, Hooked{ID: 42, TmdbID: heat})
		if len(enqueued) != 0 {
			t.Fatalf("enqueued = %d, want nothing for a movie the downloader no longer has", len(enqueued))
		}

		if fixture.item(t, items.MovieKey(heat)).DeletedAt == nil {
			t.Error("Heat is still listed after Radarr deleted it")
		}
		if fixture.item(t, items.MovieKey(matrix)).DeletedAt != nil {
			t.Error("The Matrix was swept by a delete that named Heat")
		}
		if held := fixture.paths(t); len(held) != 1 {
			t.Errorf("files = %v, want only The Matrix's", held)
		}
	})

	t.Run("a delete with no TMDB id leaves everything for the next refresh", func(t *testing.T) {
		if enqueued := fixture.hooked(t, fixture.service, Hooked{ID: 99}); len(enqueued) != 0 {
			t.Errorf("enqueued = %d, want nothing", len(enqueued))
		}
		if held := fixture.paths(t); len(held) != 1 {
			t.Errorf("files = %v, want The Matrix's left alone", held)
		}
	})
}

func TestAuthenticate(t *testing.T) {
	reported := []movie{}
	fixture := newFixture(t, &reported)
	ctx := context.Background()

	source, err := fixture.client.Source.Get(ctx, fixture.sourceID)
	if err != nil {
		t.Fatalf("failed to read the source: %v", err)
	}
	if len(source.WebhookSecret) < 20 {
		t.Fatalf("secret = %q, want one made when the source was", source.WebhookSecret)
	}

	for _, test := range []struct {
		name     string
		id       uuid.UUID
		username string
		password string
		want     error
	}{
		{"the source's secret", fixture.sourceID, WebhookUsername, source.WebhookSecret, nil},
		{"a wrong secret", fixture.sourceID, WebhookUsername, "guess", ErrUnauthorized},
		{"no secret", fixture.sourceID, "", "", ErrUnauthorized},
		{"another username", fixture.sourceID, "admin", source.WebhookSecret, ErrUnauthorized},
		{"a source that does not exist", uuid.New(), WebhookUsername, source.WebhookSecret, ErrUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := fixture.service.Authenticate(ctx, test.id, test.username, test.password); !errors.Is(err, test.want) {
				t.Errorf("err = %v, want %v", err, test.want)
			}
		})
	}
}

func TestHeard(t *testing.T) {
	radarr, sonarr, lidarr := Source{Kind: KindRadarr}, Source{Kind: KindSonarr}, Source{Kind: KindLidarr}

	for _, test := range []struct {
		name   string
		source Source
		body   string
		want   *Hooked
	}{
		{"the test a save sends", radarr, `{"eventType":"Test","movie":{"id":1}}`, nil},
		{"a grab, which has no file yet", radarr, `{"eventType":"Grab","movie":{"id":1,"tmdbId":603}}`, nil},
		{"an import", radarr, `{"eventType":"Download","isUpgrade":true,"movie":{"id":7,"tmdbId":603}}`, &Hooked{ID: 7, TmdbID: 603}},
		{"a movie deleted", radarr, `{"eventType":"MovieDelete","movie":{"id":7,"tmdbId":603}}`, &Hooked{ID: 7, TmdbID: 603}},
		{"an episode imported", sonarr, `{"eventType":"Download","series":{"id":3,"tmdbId":1396},"episodes":[{"id":9}]}`, &Hooked{ID: 3, TmdbID: 1396}},
		{"a series deleted", sonarr, `{"eventType":"SeriesDelete","series":{"id":3,"tmdbId":1396}}`, &Hooked{ID: 3, TmdbID: 1396}},
		{"a series event sent to a Radarr", radarr, `{"eventType":"Download","series":{"id":3}}`, nil},
		{"a health warning", sonarr, `{"eventType":"Health"}`, nil},
		{"an album imported", lidarr, `{"eventType":"Download","artist":{"id":5,"mbId":"A-ONE"},"albums":[{"id":8}]}`, &Hooked{ID: 5, MusicBrainzID: "A-ONE"}},
		{"an artist deleted", lidarr, `{"eventType":"ArtistDelete","artist":{"id":5,"mbId":"A-ONE"}}`, &Hooked{ID: 5, MusicBrainzID: "A-ONE"}},
		{"an album deleted", lidarr, `{"eventType":"AlbumDelete","artist":{"id":5,"mbId":"A-ONE"},"album":{"id":8}}`, &Hooked{ID: 5, MusicBrainzID: "A-ONE"}},
		{"a retag, which moves no file", lidarr, `{"eventType":"Retag","artist":{"id":5}}`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			hooked, refreshes, err := Heard(test.source, strings.NewReader(test.body))
			if err != nil {
				t.Fatalf("failed to read the event: %v", err)
			}
			if refreshes != (test.want != nil) {
				t.Fatalf("refreshes = %v, want %v", refreshes, test.want != nil)
			}
			if test.want != nil && (hooked.ID != test.want.ID || hooked.TmdbID != test.want.TmdbID || hooked.MusicBrainzID != test.want.MusicBrainzID) {
				t.Errorf("hooked = %+v, want %+v", hooked, *test.want)
			}
		})
	}

	if _, _, err := Heard(radarr, strings.NewReader("not json")); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid for a body that is not an event", err)
	}
}

func TestHook(t *testing.T) {
	reported := []movie{}
	fixture := newFixture(t, &reported)
	ctx := context.Background()

	t.Run("with no published address it says why and registers nothing", func(t *testing.T) {
		if err := fixture.service.Hook(ctx, fixture.sourceID); !errors.Is(err, errUnpublished) {
			t.Fatalf("err = %v, want errUnpublished", err)
		}

		source, err := fixture.client.Source.Get(ctx, fixture.sourceID)
		if err != nil {
			t.Fatalf("failed to read the source: %v", err)
		}
		if source.WebhookError == "" || source.WebhookRegisteredAt != nil {
			t.Errorf("source = %q at %v, want the reason and no registration", source.WebhookError, source.WebhookRegisteredAt)
		}
		if held := fixture.downloader.held(); len(held) != 0 {
			t.Errorf("notifications = %v, want none", held)
		}
	})

	published := fixture.published(t)

	t.Run("registers a webhook from the downloader's own schema", func(t *testing.T) {
		if err := published.Hook(ctx, fixture.sourceID); err != nil {
			t.Fatalf("failed to register: %v", err)
		}

		source, err := fixture.client.Source.Get(ctx, fixture.sourceID)
		if err != nil {
			t.Fatalf("failed to read the source: %v", err)
		}
		if source.WebhookRegisteredAt == nil || source.WebhookError != "" {
			t.Errorf("source = %q at %v, want it registered", source.WebhookError, source.WebhookRegisteredAt)
		}

		held := fixture.downloader.held()
		if len(held) != 1 {
			t.Fatalf("notifications = %d, want one", len(held))
		}
		hook := held[0]
		if hook["name"] != webhookName || hook["implementation"] != webhookImplementation {
			t.Errorf("notification = %v, want gojellyfin's webhook", hook)
		}
		for event, want := range map[string]bool{
			"onDownload": true, "onUpgrade": true, "onRename": true, "onMovieDelete": true,
			"onMovieFileDelete": false, "onGrab": false, "onHealthIssue": false,
		} {
			if hook[event] != want {
				t.Errorf("%s = %v, want %v", event, hook[event], want)
			}
		}
		if _, invented := hook["onSeriesDelete"]; invented {
			t.Error("onSeriesDelete was added to a Radarr that does not offer it")
		}

		fields := map[string]any{}
		for _, field := range hook["fields"].([]any) {
			entry := field.(map[string]any)
			fields[entry["name"].(string)] = entry["value"]
		}
		want := map[string]any{
			"url":      "https://jellyfin.example/webhooks/" + fixture.sourceID.String(),
			"method":   float64(webhookMethodPost),
			"username": WebhookUsername,
			"password": source.WebhookSecret,
		}
		for name, value := range want {
			if fields[name] != value {
				t.Errorf("%s = %v, want %v", name, fields[name], value)
			}
		}
	})

	t.Run("a second save replaces it rather than adding another", func(t *testing.T) {
		if err := published.Hook(ctx, fixture.sourceID); err != nil {
			t.Fatalf("failed to register again: %v", err)
		}
		if held := fixture.downloader.held(); len(held) != 1 {
			t.Errorf("notifications = %d, want still one", len(held))
		}
		if methods := fixture.downloader.methods; methods[len(methods)-1] != http.MethodPut {
			t.Errorf("methods = %v, want the second registration to be a PUT", methods)
		}
	})

	t.Run("a duplicate left on the downloader is removed", func(t *testing.T) {
		fixture.downloader.Lock()
		duplicate := map[string]any{}
		for key, value := range fixture.downloader.notifications[0] {
			duplicate[key] = value
		}
		fixture.downloader.nextID++
		duplicate["id"] = float64(fixture.downloader.nextID)
		fixture.downloader.notifications = append(fixture.downloader.notifications, duplicate)
		fixture.downloader.Unlock()

		if err := published.Hook(ctx, fixture.sourceID); err != nil {
			t.Fatalf("failed to register again: %v", err)
		}
		if held := fixture.downloader.held(); len(held) != 1 {
			t.Errorf("notifications = %d, want the duplicate gone", len(held))
		}
	})

	t.Run("registrations racing on two replicas add one webhook", func(t *testing.T) {
		fixture.downloader.Lock()
		fixture.downloader.notifications = nil
		fixture.downloader.Unlock()

		var group sync.WaitGroup
		for range 4 {
			group.Add(1)
			go func() {
				defer group.Done()
				if err := published.Hook(ctx, fixture.sourceID); err != nil {
					t.Errorf("failed to register: %v", err)
				}
			}()
		}
		group.Wait()

		if held := fixture.downloader.held(); len(held) != 1 {
			t.Errorf("notifications = %d, want one", len(held))
		}
	})

	t.Run("deleting the source takes it off the downloader", func(t *testing.T) {
		source, err := fixture.client.Source.Query().Where(sourcemodel.ID(fixture.sourceID)).OnlyModel(ctx)
		if err != nil {
			t.Fatalf("failed to read the source: %v", err)
		}
		if err := published.unhook(ctx, *source); err != nil {
			t.Fatalf("failed to remove the webhook: %v", err)
		}
		if held := fixture.downloader.held(); len(held) != 0 {
			t.Errorf("notifications = %v, want none", held)
		}
	})
}
