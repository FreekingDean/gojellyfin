package items

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func tmdbID() int {
	return int(uuid.New().ID())
}

func (f *fixture) refresh(t *testing.T, scanned Scanned) []jobs.Queued {
	t.Helper()

	enqueued, err := jobs.RunJob(t, f.service.RefreshItemJob(),
		jobs.With(jobs.ParamItem, scanned),
		jobs.With(jobs.ParamLibrary, f.libraryID),
		jobs.With(jobs.ParamSource, f.downloader(t)),
	)
	if err != nil {
		t.Fatalf("failed to refresh %s: %v", scanned.Key, err)
	}

	return enqueued
}

func TestService_RefreshItem(t *testing.T) {
	t.Run("asks for a probe and metadata for a title it has just written", func(t *testing.T) {
		fixture := newFixture(t)

		enqueued := fixture.refresh(t, Scanned{
			Key:      MovieKey(tmdbID()),
			Kind:     itemmodel.KindMovie,
			Name:     "The Matrix",
			SortName: "matrix",
			Files: []ScannedFile{{
				Path:         "/media/The Matrix/" + uuid.NewString() + ".mkv",
				DateModified: time.Now(),
			}},
		})

		if probes := jobs.Enqueued(t, enqueued, jobs.ProbeFile); len(probes) != 1 {
			t.Errorf("probes = %d, want the new file probed", len(probes))
		}
		if identify := jobs.Enqueued(t, enqueued, jobs.RefreshItemMetadata); len(identify) != 1 {
			t.Errorf("metadata = %d, want the unidentified title identified", len(identify))
		}
	})

	t.Run("asks for neither once the file is probed and the title identified", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		path := "/media/The Matrix/" + uuid.NewString() + ".mkv"

		scanned := Scanned{
			Key:      MovieKey(tmdbID()),
			Kind:     itemmodel.KindMovie,
			Name:     "The Matrix",
			SortName: "matrix",
			Files:    []ScannedFile{{Path: path, DateModified: time.Now()}},
		}
		fixture.refresh(t, scanned)

		item, err := fixture.service.ItemByKey(ctx, scanned.Key)
		if err != nil {
			t.Fatalf("failed to read the item back: %v", err)
		}

		sources, err := fixture.service.MediaSources(ctx, item.ID)
		if err != nil {
			t.Fatalf("failed to read the sources back: %v", err)
		}
		if err := fixture.service.SaveProbe(ctx, item, sources[0], MediaSource{Container: "mkv"}); err != nil {
			t.Fatalf("failed to probe: %v", err)
		}
		if _, err := fixture.service.UpdateMetadata(ctx, item.ID, Metadata{
			ProviderIds: &map[string]string{"Tmdb": "603"},
		}); err != nil {
			t.Fatalf("failed to identify: %v", err)
		}

		enqueued := fixture.refresh(t, scanned)

		if probes := jobs.Enqueued(t, enqueued, jobs.ProbeFile); len(probes) != 0 {
			t.Errorf("probes = %d, want the probed file left alone", len(probes))
		}
		if identify := jobs.Enqueued(t, enqueued, jobs.RefreshItemMetadata); len(identify) != 0 {
			t.Errorf("metadata = %d, want the identified title left alone", len(identify))
		}
	})

	t.Run("deletes the title a file was moved off when it has no file left", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		path := "/media/Renumbered/" + uuid.NewString() + ".mkv"

		before := Scanned{
			Key:      MovieKey(tmdbID()),
			Kind:     itemmodel.KindMovie,
			Name:     "Renumbered",
			SortName: "renumbered",
			Files:    []ScannedFile{{Path: path, DateModified: time.Now()}},
		}
		fixture.refresh(t, before)

		after := before
		after.Key = MovieKey(tmdbID())
		fixture.refresh(t, after)

		left, err := fixture.service.ItemByKey(ctx, before.Key)
		if err != nil {
			t.Fatalf("failed to read the old title back: %v", err)
		}
		if left.DeletedAt == nil {
			t.Error("the title the file moved off is still live with nothing to play")
		}

		took, err := fixture.service.ItemByKey(ctx, after.Key)
		if err != nil {
			t.Fatalf("failed to read the new title back: %v", err)
		}
		if took.DeletedAt != nil {
			t.Error("the title the file moved onto was deleted")
		}
	})

	t.Run("asks to listen for the intro of the season a new episode lands in", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()
		show := tmdbID()
		one := int32(1)

		fixture.refresh(t, Scanned{Key: SeriesKey(show), Kind: itemmodel.KindSeries, Name: "Show", SortName: "show"})
		fixture.refresh(t, Scanned{
			Key: SeasonKey(show, 1), ParentKey: SeriesKey(show), Kind: itemmodel.KindSeason,
			Name: "Season 1", SortName: "season 1", Index: &one,
		})
		enqueued := fixture.refresh(t, Scanned{
			Key: EpisodeKey(show, 1, 1), ParentKey: SeasonKey(show, 1), Kind: itemmodel.KindEpisode,
			Name: "Pilot", SortName: "pilot", Index: &one, ParentIndex: &one,
			Files: []ScannedFile{{Path: "/media/Show/" + uuid.NewString() + ".mkv", DateModified: time.Now()}},
		})

		season, err := fixture.service.ItemByKey(ctx, SeasonKey(show, 1))
		if err != nil {
			t.Fatal(err)
		}
		asked := jobs.Enqueued(t, enqueued, jobs.DetectSeasonIntros)
		if len(asked) != 1 || asked[0][jobs.ParamSeason] != `"`+season.ID.String()+`"` {
			t.Errorf("detections = %v, want one for season %s", asked, season.ID)
		}

		movie := fixture.refresh(t, Scanned{
			Key: MovieKey(tmdbID()), Kind: itemmodel.KindMovie, Name: "Film", SortName: "film",
			Files: []ScannedFile{{Path: "/media/Film/" + uuid.NewString() + ".mkv", DateModified: time.Now()}},
		})
		if asked := jobs.Enqueued(t, movie, jobs.DetectSeasonIntros); len(asked) != 0 {
			t.Errorf("detections = %d, want none for a movie", len(asked))
		}
	})
}
