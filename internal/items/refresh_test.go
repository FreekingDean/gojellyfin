package items

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
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
			Kind:     itemmodal.KindMovie,
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
			Kind:     itemmodal.KindMovie,
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
}
