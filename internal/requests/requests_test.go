package requests

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	requestmodel "github.com/FreekingDean/gojellyfin/internal/store/request"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

var errRefused = errors.New("the instance refused")

type stubDownloaders struct {
	targets   []sources.Source
	refusing  bool
	submitted []string
	wanted    []sources.Wanted
}

func (s *stubDownloaders) Requestable(_ context.Context, kind sources.Kind) ([]sources.Source, error) {
	matching := []sources.Source{}
	for _, target := range s.targets {
		if target.Kind == kind {
			matching = append(matching, target)
		}
	}

	return matching, nil
}

func (s *stubDownloaders) Submit(_ context.Context, source sources.Source, wanted sources.Wanted) error {
	s.submitted = append(s.submitted, source.Name)
	s.wanted = append(s.wanted, wanted)
	if s.refusing {
		return errRefused
	}

	return nil
}

type fixture struct {
	service     *Service
	client      *store.Client
	downloaders *stubDownloaders
	users       []uuid.UUID
	radarr      sources.Source
	sonarr      sources.Source
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()

	connection := storetest.Open(t)
	t.Cleanup(func() {
	})
	client := connection.Client()

	source := func(kind sources.Kind) sources.Source {
		created, err := client.Source.Create().
			SetName(t.Name() + "-" + uuid.NewString()).
			SetURL("http://" + uuid.NewString() + ".invalid").
			SetAPIKeyVariable("SOURCE_API_KEY_TEST").
			SetKind(kind).
			SaveModel(ctx)
		if err != nil {
			t.Fatalf("failed to create the source: %v", err)
		}
		t.Cleanup(func() {
			if err := client.Source.DeleteOneID(created.ID).Exec(context.Background()); err != nil {
				t.Errorf("failed to delete the source: %v", err)
			}
		})

		return *created
	}

	users := make([]uuid.UUID, 2)
	for i := range users {
		name := t.Name() + "-" + uuid.NewString()
		user, err := client.User.Create().SetName(name).SetUsername(name).SetPasswordHash("hash").Save(ctx)
		if err != nil {
			t.Fatalf("failed to create the user: %v", err)
		}
		t.Cleanup(func() {
			if err := client.User.DeleteOne(user).Exec(context.Background()); err != nil {
				t.Errorf("failed to delete the user: %v", err)
			}
		})
		users[i] = user.ID
	}

	radarr, sonarr := source(sourcemodel.KindRadarr), source(sourcemodel.KindSonarr)
	downloaders := &stubDownloaders{targets: []sources.Source{radarr, sonarr}}

	return &fixture{
		service:     newService(client, downloaders),
		client:      client,
		downloaders: downloaders,
		users:       users,
		radarr:      radarr,
		sonarr:      sonarr,
	}
}

func wanted(kind items.Kind, ids map[string]string) discover.Title {
	tmdbID := uint32(900000000 + uuid.New().ID()%100000000)
	name := "Wanted"

	return discover.Title{
		ID:       discover.ID(kind, tmdbID),
		Kind:     kind,
		TmdbID:   tmdbID,
		Key:      discover.Key(kind, tmdbID),
		Metadata: items.Metadata{Name: &name, ProviderIds: &ids},
	}
}

func (f *fixture) mark(t *testing.T, userID uuid.UUID, key string) Mark {
	t.Helper()

	marks, err := f.service.Marks(context.Background(), userID, []string{key})
	if err != nil {
		t.Fatalf("failed to read the marks: %v", err)
	}

	return marks[key]
}

func TestService_Request(t *testing.T) {
	ctx := context.Background()

	t.Run("a movie goes to the Radarr and is marked for whoever asked", func(t *testing.T) {
		fixture := newFixture(t)
		title := wanted(itemmodel.KindMovie, nil)
		asker, other := fixture.users[0], fixture.users[1]

		if err := fixture.service.Request(ctx, asker, title); err != nil {
			t.Fatalf("failed to request: %v", err)
		}

		if got := fixture.downloaders.submitted; !slices.Equal(got, []string{fixture.radarr.Name}) {
			t.Errorf("submitted to %v, want the Radarr alone", got)
		}
		if got := fixture.downloaders.wanted[0]; got.TmdbID != int(title.TmdbID) || got.Name != "Wanted" {
			t.Errorf("wanted = %+v", got)
		}
		if got := fixture.mark(t, asker, title.Key); !got.Requested || !got.Mine {
			t.Errorf("the asker's mark = %+v", got)
		}
		if got := fixture.mark(t, other, title.Key); !got.Requested || got.Mine {
			t.Errorf("another user's mark = %+v", got)
		}

		submissions, err := fixture.service.Submissions(ctx, title.Key)
		if err != nil {
			t.Fatalf("failed to read the submissions: %v", err)
		}
		if len(submissions) != 1 || submissions[0].SourceID != fixture.radarr.ID || submissions[0].AcceptedAt == nil || submissions[0].LastError != "" {
			t.Errorf("submissions = %+v", submissions)
		}
	})

	t.Run("a series goes to the Sonarr with its TVDB id", func(t *testing.T) {
		fixture := newFixture(t)
		title := wanted(itemmodel.KindSeries, map[string]string{"Tvdb": "81189"})

		if err := fixture.service.Request(ctx, fixture.users[0], title); err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		if got := fixture.downloaders.submitted; !slices.Equal(got, []string{fixture.sonarr.Name}) {
			t.Errorf("submitted to %v, want the Sonarr alone", got)
		}
		if got := fixture.downloaders.wanted[0].TvdbID; got != 81189 {
			t.Errorf("tvdb id = %d, want 81189", got)
		}
	})

	t.Run("a second asker is recorded and the instance is not asked twice", func(t *testing.T) {
		fixture := newFixture(t)
		title := wanted(itemmodel.KindMovie, nil)

		for _, user := range []uuid.UUID{fixture.users[0], fixture.users[1], fixture.users[0]} {
			if err := fixture.service.Request(ctx, user, title); err != nil {
				t.Fatalf("failed to request: %v", err)
			}
		}

		if got := len(fixture.downloaders.submitted); got != 1 {
			t.Errorf("submitted %d times, want once", got)
		}
		count, err := fixture.client.Request.Query().Where(requestmodel.Key(title.Key)).Count(ctx)
		if err != nil || count != 2 {
			t.Errorf("requests = %d, %v, want one per asker", count, err)
		}
	})

	t.Run("an instance that refuses keeps the request and is asked again next time", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.downloaders.refusing = true
		title := wanted(itemmodel.KindMovie, nil)

		if err := fixture.service.Request(ctx, fixture.users[0], title); err != nil {
			t.Fatalf("a refused submission failed the request: %v", err)
		}
		if got := fixture.mark(t, fixture.users[0], title.Key); !got.Mine {
			t.Error("the request was not kept")
		}
		submissions, _ := fixture.service.Submissions(ctx, title.Key)
		if len(submissions) != 1 || submissions[0].AcceptedAt != nil || submissions[0].LastError != errRefused.Error() {
			t.Fatalf("submissions = %+v", submissions)
		}

		fixture.downloaders.refusing = false
		if err := fixture.service.Request(ctx, fixture.users[1], title); err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		submissions, _ = fixture.service.Submissions(ctx, title.Key)
		if len(submissions) != 1 || submissions[0].AcceptedAt == nil || submissions[0].LastError != "" {
			t.Errorf("submissions after the retry = %+v", submissions)
		}
	})

	t.Run("with no source taking requests nothing is recorded", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.downloaders.targets = nil
		title := wanted(itemmodel.KindMovie, nil)

		if err := fixture.service.Request(ctx, fixture.users[0], title); !errors.Is(err, ErrNowhere) {
			t.Errorf("err = %v, want ErrNowhere", err)
		}
		if got := fixture.mark(t, fixture.users[0], title.Key); got.Requested {
			t.Error("a request nobody can take was recorded")
		}
	})
}

func TestService_Withdraw(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t)
	title := wanted(itemmodel.KindMovie, nil)
	asker, other := fixture.users[0], fixture.users[1]

	for _, user := range fixture.users {
		if err := fixture.service.Request(ctx, user, title); err != nil {
			t.Fatalf("failed to request: %v", err)
		}
	}

	if err := fixture.service.Withdraw(ctx, asker, title.Key); err != nil {
		t.Fatalf("failed to withdraw: %v", err)
	}
	if got := fixture.mark(t, asker, title.Key); got.Mine || !got.Requested {
		t.Errorf("the withdrawn asker's mark = %+v, want the other's request still showing", got)
	}
	if got := fixture.mark(t, other, title.Key); !got.Mine {
		t.Error("withdrawing took another user's request")
	}

	if err := fixture.service.Withdraw(ctx, asker, title.Key); err != nil {
		t.Errorf("withdrawing twice failed: %v", err)
	}
}
