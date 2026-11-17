package items

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_Mix(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	rock, jazz := "Rock "+uuid.NewString(), "Jazz "+uuid.NewString()

	artist := serviceMixArtist(t, fixture, ctx)
	track := func(name string, album uuid.UUID) uuid.UUID {
		return fixture.add(t, seed{kind: itemmodel.KindAudio, name: name, parentID: &album})
	}

	_, first := artist("First", rock)
	_, second := artist("Second", rock)
	_, third := artist("Third", jazz)
	opener, closer, cover, standard := track("Opener", first), track("Closer", first), track("Cover", second), track("Standard", third)

	viewer := Viewer{Libraries: []uuid.UUID{fixture.libraryID}}
	ids := func(records []*Item) []uuid.UUID {
		found := make([]uuid.UUID, 0, len(records))
		for _, record := range records {
			found = append(found, record.ID)
		}
		slices.SortFunc(found, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })

		return found
	}
	want := func(of ...uuid.UUID) []uuid.UUID {
		slices.SortFunc(of, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })

		return of
	}

	fromSong, err := fixture.service.Mix(ctx, viewer, opener, 0)
	if err != nil {
		t.Fatalf("failed to mix from a song: %v", err)
	}
	if got := ids(fromSong); !slices.Equal(got, want(opener, closer, cover)) {
		t.Errorf("from a song = %v, want the artist's tracks and the other rock", got)
	}

	limited, err := fixture.service.Mix(ctx, viewer, first, 1)
	if err != nil || len(limited) != 1 {
		t.Errorf("limited = %d (%v), want one", len(limited), err)
	}

	byGenre, err := fixture.service.GenreMix(ctx, viewer, nil, jazz, 0)
	if err != nil {
		t.Fatalf("failed to mix a genre: %v", err)
	}
	if got := ids(byGenre); !slices.Equal(got, want(standard)) {
		t.Errorf("by genre = %v, want the jazz", got)
	}

	movie := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Film"})
	if none, err := fixture.service.Mix(ctx, viewer, movie, 0); err != nil || len(none) != 0 {
		t.Errorf("from a film = %d (%v), want nothing to mix", len(none), err)
	}
	if _, err := fixture.service.Mix(ctx, viewer, uuid.New(), 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func serviceMixArtist(t *testing.T, fixture *fixture, ctx context.Context) func(name, genre string) (uuid.UUID, uuid.UUID) {
	t.Helper()

	return func(name, genre string) (uuid.UUID, uuid.UUID) {
		t.Helper()

		id := fixture.add(t, seed{kind: itemmodel.KindMusicArtist, name: name})
		genres := []string{genre}
		if err := fixture.service.replaceNamed(ctx, id, Metadata{Genres: &genres}); err != nil {
			t.Fatalf("failed to tag %s: %v", name, err)
		}

		return id, fixture.add(t, seed{kind: itemmodel.KindMusicAlbum, name: name + " Album", parentID: &id})
	}
}
