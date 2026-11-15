package items

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"

	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
)

func (f *fixture) paths(t *testing.T, downloader uuid.UUID) []string {
	t.Helper()

	held, err := f.service.store.ItemSource.Query().
		Where(sourcemodel.SourceID(downloader)).
		Select(sourcemodel.FieldPath).
		Strings(context.Background())
	if err != nil {
		t.Fatalf("failed to read the files back: %v", err)
	}
	slices.Sort(held)

	return held
}

func TestService_DeleteScopedSourcesNotInPaths(t *testing.T) {
	fixture := newFixture(t)
	downloader := fixture.downloader(t)
	series := tmdbID()
	neighbour := series*10 + 1

	at := func(name string) string { return fmt.Sprintf("/media/%d/%s", series, name) }

	fixture.scannedFrom(t, downloader, SeriesKey(series), at("show/series.nfo"))
	fixture.scannedFrom(t, downloader, EpisodeKey(series, 1, 1), at("show/s01e01.mkv"))
	fixture.scannedFrom(t, downloader, EpisodeKey(series, 1, 2), at("show/s01e02.mkv"))
	fixture.scannedFrom(t, downloader, EpisodeKey(neighbour, 1, 1), at("neighbour/s01e01.mkv"))
	fixture.scannedFrom(t, downloader, MovieKey(series), at("film.mkv"))

	dropped, err := fixture.service.DeleteScopedSourcesNotInPaths(
		context.Background(), downloader, SeriesScope(series), []string{at("show/s01e02.mkv")},
	)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if len(dropped) != 2 {
		t.Errorf("dropped = %d, want the series' and the first episode's files", len(dropped))
	}

	want := []string{at("film.mkv"), at("neighbour/s01e01.mkv"), at("show/s01e02.mkv")}
	if got := fixture.paths(t, downloader); !slices.Equal(got, want) {
		t.Errorf("files = %v, want %v: a series' scope reaches neither a series whose id begins with its own nor a film sharing its id", got, want)
	}
}
