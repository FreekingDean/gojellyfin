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

func TestService_DeleteScopedSourcesNotInPaths_Artist(t *testing.T) {
	fixture := newFixture(t)
	downloader := fixture.downloader(t)
	ctx := context.Background()
	band, neighbour := uuid.NewString(), uuid.NewString()

	under := func(parent *Item, key, path string) *Item {
		t.Helper()

		saved := fixture.scannedFrom(t, downloader, key, path)
		if err := fixture.service.store.Item.UpdateOneID(saved.ID).SetParentID(parent.ID).Exec(ctx); err != nil {
			t.Fatalf("failed to hang %s beneath its parent: %v", key, err)
		}

		return saved
	}

	at := func(name string) string { return "/media/music/" + band + "/" + name }

	artist := fixture.scannedFrom(t, downloader, ArtistKey(band), at("artist.nfo"))
	album := under(artist, AlbumKey(uuid.NewString()), at("album/cover.jpg"))
	under(album, TrackKey(uuid.NewString()), at("album/01.flac"))
	under(album, TrackKey(uuid.NewString()), at("album/02.flac"))
	other := fixture.scannedFrom(t, downloader, ArtistKey(neighbour), at("other/artist.nfo"))
	under(other, AlbumKey(uuid.NewString()), at("other/album.flac"))

	dropped, err := fixture.service.DeleteScopedSourcesNotInPaths(ctx, downloader, ArtistScope(band), []string{at("album/02.flac")})
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if len(dropped) != 3 {
		t.Errorf("dropped = %d, want the artist's, the album's and the first track's files", len(dropped))
	}

	want := []string{at("album/02.flac"), at("other/album.flac"), at("other/artist.nfo")}
	if got := fixture.paths(t, downloader); !slices.Equal(got, want) {
		t.Errorf("files = %v, want %v: an artist's scope reaches its albums and tracks and nobody else's", got, want)
	}
}
