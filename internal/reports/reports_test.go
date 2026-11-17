package reports

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type stubDownloaders struct {
	replaced []sources.Copy
	failing  error
}

func (s *stubDownloaders) Replace(_ context.Context, _ uuid.UUID, broken sources.Copy) error {
	s.replaced = append(s.replaced, broken)

	return s.failing
}

type fixture struct {
	service     *Service
	client      *store.Client
	downloaders *stubDownloaders
	user        uuid.UUID
	source      uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()

	connection := storetest.Open(t)
	t.Cleanup(func() {
	})
	client := connection.Client()

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

	source, err := client.Source.Create().
		SetName(name).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindRadarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Source.DeleteOne(source).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	downloaders := &stubDownloaders{}

	return &fixture{
		service:     newService(client, downloaders),
		client:      client,
		downloaders: downloaders,
		user:        user.ID,
		source:      source.ID,
	}
}

func (f *fixture) movie(t *testing.T, named ...sources.Copy) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	name := t.Name() + "-" + uuid.NewString()
	movie, err := f.client.Item.Create().SetKind(itemmodel.KindMovie).SetKey(name).SetName(name).SetSortName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the movie: %v", err)
	}
	t.Cleanup(func() {
		if err := f.client.Item.DeleteOne(movie).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the movie: %v", err)
		}
	})

	for _, file := range named {
		err := f.client.ItemSource.Create().
			SetItemID(movie.ID).
			SetSourceID(f.source).
			SetName(name).
			SetPath("/" + name + "/" + uuid.NewString()).
			SetExternalID(file.TitleID).
			SetExternalFileID(file.FileID).
			Exec(ctx)
		if err != nil {
			t.Fatalf("failed to create the file: %v", err)
		}
	}

	return movie.ID
}

func (f *fixture) reported(t *testing.T, itemID uuid.UUID) []*Report {
	t.Helper()

	queue, err := f.service.Queue(context.Background())
	if err != nil {
		t.Fatalf("failed to read the queue: %v", err)
	}

	return slices.DeleteFunc(queue, func(filed *Report) bool { return filed.ItemID != itemID })
}

func TestService_File(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t)
	movie := fixture.movie(t)
	filing := Filing{ItemID: movie, UserID: &fixture.user, Kind: KindAudio, Origin: OriginUser, Note: "no sound after the first hour"}

	for range 2 {
		if err := fixture.service.File(ctx, filing); err != nil {
			t.Fatalf("failed to file the report: %v", err)
		}
	}

	filed := fixture.reported(t, movie)
	if len(filed) != 1 {
		t.Fatalf("got %d reports, want the one both filings share", len(filed))
	}
	if filed[0].Status != StatusOpen || filed[0].Note != filing.Note || filed[0].User == nil || filed[0].Item == nil {
		t.Errorf("report = %+v", filed[0])
	}
	if Replaceable(filed[0]) {
		t.Errorf("a title no downloader names a file for is offered a replacement")
	}

	automatic := Filing{ItemID: movie, Kind: KindUnplayable, Origin: OriginProbe}
	if err := fixture.service.File(ctx, automatic); err != nil {
		t.Fatalf("failed to file the automatic report: %v", err)
	}
	if got := len(fixture.reported(t, movie)); got != 2 {
		t.Errorf("got %d reports, want the user's and the probe's", got)
	}

	if err := fixture.service.Dismiss(ctx, filed[0].ID); err != nil {
		t.Fatalf("failed to dismiss: %v", err)
	}
	if err := fixture.service.File(ctx, filing); err != nil {
		t.Fatalf("failed to file again: %v", err)
	}
	if got := len(fixture.reported(t, movie)); got != 3 {
		t.Errorf("got %d reports, want a fresh one once the last was dismissed", got)
	}

	if err := fixture.service.File(ctx, Filing{ItemID: uuid.New(), Kind: KindOther, Origin: OriginUser}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a report of nothing = %v, want ErrNotFound", err)
	}
}

func TestService_Replace(t *testing.T) {
	ctx := context.Background()

	t.Run("the named file is replaced and the title's open reports resolved", theNamedFileIsReplacedAndTheTitlesOpenReportsResolved(ctx))

	t.Run("a title nobody names a file for has nothing to replace", func(t *testing.T) {
		fixture := newFixture(t)
		movie := fixture.movie(t)
		if err := fixture.service.File(ctx, Filing{ItemID: movie, Kind: KindUnplayable, Origin: OriginProbe}); err != nil {
			t.Fatalf("failed to file the report: %v", err)
		}

		filed := fixture.reported(t, movie)
		if err := fixture.service.Replace(ctx, filed[0].ID); !errors.Is(err, ErrNothingToReplace) {
			t.Errorf("err = %v, want ErrNothingToReplace", err)
		}
	})

	t.Run("a downloader that refuses leaves the report open", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.downloaders.failing = errors.New("refused")
		movie := fixture.movie(t, sources.Copy{TitleID: 1, FileID: 2})
		if err := fixture.service.File(ctx, Filing{ItemID: movie, Kind: KindUnplayable, Origin: OriginProbe}); err != nil {
			t.Fatalf("failed to file the report: %v", err)
		}

		filed := fixture.reported(t, movie)
		if err := fixture.service.Replace(ctx, filed[0].ID); err == nil {
			t.Fatalf("a refused replacement answered success")
		}
		if got := fixture.reported(t, movie)[0].Status; got != StatusOpen {
			t.Errorf("status = %s, want Open", got)
		}
	})

	t.Run("a report that is not there is not found", func(t *testing.T) {
		fixture := newFixture(t)
		for name, decide := range map[string]func(context.Context, uuid.UUID) error{
			"replace": fixture.service.Replace,
			"resolve": fixture.service.Resolve,
			"dismiss": fixture.service.Dismiss,
		} {
			if err := decide(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
				t.Errorf("%s = %v, want ErrNotFound", name, err)
			}
		}
	})
}

func theNamedFileIsReplacedAndTheTitlesOpenReportsResolved(ctx context.Context) func(*testing.T) {
	return func(t *testing.T) {
		fixture := newFixture(t)
		copies := []sources.Copy{{TitleID: 12, FileID: 34}}
		movie := fixture.movie(t, copies...)

		for _, kind := range []Kind{KindVideo, KindAudio} {
			if err := fixture.service.File(ctx, Filing{ItemID: movie, UserID: &fixture.user, Kind: kind, Origin: OriginUser}); err != nil {
				t.Fatalf("failed to file the report: %v", err)
			}
		}
		filed := fixture.reported(t, movie)
		if !Replaceable(filed[0]) {
			t.Fatalf("a title with a named file is not offered a replacement")
		}

		if err := fixture.service.Replace(ctx, filed[0].ID); err != nil {
			t.Fatalf("failed to replace: %v", err)
		}

		if !slices.Equal(fixture.downloaders.replaced, copies) {
			t.Errorf("replaced %v, want %v", fixture.downloaders.replaced, copies)
		}
		for _, report := range fixture.reported(t, movie) {
			if report.Status != StatusResolved {
				t.Errorf("a %s report is still %s", report.Kind, report.Status)
			}
		}
	}
}
