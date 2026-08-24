package items

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	creditmodal "github.com/FreekingDean/gojellyfin/internal/store/credit"
	genremodal "github.com/FreekingDean/gojellyfin/internal/store/genre"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	personmodal "github.com/FreekingDean/gojellyfin/internal/store/person"
	studiomodal "github.com/FreekingDean/gojellyfin/internal/store/studio"
)

type metadataFixture struct {
	*fixture
	prefix string
}

func newMetadataFixture(t *testing.T) *metadataFixture {
	t.Helper()

	base := newFixture(t)
	prefix := uuid.NewString() + " "
	client := base.service.store

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Credit.Delete().Where(creditmodal.HasPersonWith(personmodal.NameHasPrefix(prefix))).Exec(ctx); err != nil {
			t.Errorf("failed to delete the credits: %v", err)
		}
		if _, err := client.Person.Delete().Where(personmodal.NameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the people: %v", err)
		}
		if _, err := client.Genre.Delete().Where(genremodal.NameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the genres: %v", err)
		}
		if _, err := client.Studio.Delete().Where(studiomodal.NameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the studios: %v", err)
		}
	})

	return &metadataFixture{fixture: base, prefix: prefix}
}

func (f *metadataFixture) item(t *testing.T, name string) *Item {
	t.Helper()

	id := f.add(t, seed{kind: itemmodal.KindMovie, name: name})
	record, err := f.service.ItemByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to read %q: %v", name, err)
	}

	return record
}

func (f *metadataFixture) query() MetadataQuery {
	return MetadataQuery{LibraryID: &f.libraryID}
}

func (f *metadataFixture) name(value string) string {
	return f.prefix + value
}

func namesOf(named []Named) []string {
	found := make([]string, 0, len(named))
	for _, one := range named {
		found = append(found, one.Name)
	}

	return found
}

func TestService_SaveProbe(t *testing.T) {
	t.Run("writes the container metadata", func(t *testing.T) {
		fixture := newMetadataFixture(t)
		ctx := context.Background()
		movie := fixture.item(t, "Movie")

		probe := Probe{
			Container: "mkv",
			Metadata: ContainerMetadata{
				Genres:  []string{fixture.name("Comedy"), fixture.name("Drama")},
				Studios: []string{fixture.name("Studio")},
				Tags:    []string{"live"},
				People: []Person{
					{Name: fixture.name("Director"), Kind: creditmodal.KindDirector},
					{Name: fixture.name("Writer"), Kind: creditmodal.KindWriter},
				},
			},
		}
		if err := fixture.service.SaveProbe(ctx, movie, probe); err != nil {
			t.Fatalf("failed to save the probe: %v", err)
		}

		t.Run("populates the genres", func(t *testing.T) {
			named, total, err := fixture.service.DistinctGenres(ctx, fixture.query())
			if err != nil {
				t.Fatalf("failed to query genres: %v", err)
			}

			want := []string{fixture.name("Comedy"), fixture.name("Drama")}
			if got := namesOf(named); !slices.Equal(got, want) {
				t.Errorf("genres = %v, want %v", got, want)
			}
			if total != 2 {
				t.Errorf("total = %d, want 2", total)
			}
		})

		t.Run("populates the studios", func(t *testing.T) {
			named, _, err := fixture.service.DistinctStudios(ctx, fixture.query())
			if err != nil {
				t.Fatalf("failed to query studios: %v", err)
			}

			want := []string{fixture.name("Studio")}
			if got := namesOf(named); !slices.Equal(got, want) {
				t.Errorf("studios = %v, want %v", got, want)
			}
		})

		t.Run("populates the people", func(t *testing.T) {
			named, _, err := fixture.service.DistinctPeople(ctx, fixture.query(), nil)
			if err != nil {
				t.Fatalf("failed to query people: %v", err)
			}

			want := []string{fixture.name("Director"), fixture.name("Writer")}
			if got := namesOf(named); !slices.Equal(got, want) {
				t.Errorf("people = %v, want %v", got, want)
			}
		})

		t.Run("filters people by credit kind", func(t *testing.T) {
			named, _, err := fixture.service.DistinctPeople(ctx, fixture.query(), []CreditKind{creditmodal.KindWriter})
			if err != nil {
				t.Fatalf("failed to query people: %v", err)
			}

			want := []string{fixture.name("Writer")}
			if got := namesOf(named); !slices.Equal(got, want) {
				t.Errorf("people = %v, want %v", got, want)
			}
		})

		t.Run("populates the tags", func(t *testing.T) {
			tags, err := fixture.service.DistinctTags(ctx, fixture.query())
			if err != nil {
				t.Fatalf("failed to query tags: %v", err)
			}

			if want := []string{"live"}; !slices.Equal(tags, want) {
				t.Errorf("tags = %v, want %v", tags, want)
			}
		})

		t.Run("re-probing changes nothing", func(t *testing.T) {
			if err := fixture.service.SaveProbe(ctx, movie, probe); err != nil {
				t.Fatalf("failed to save the probe: %v", err)
			}

			named, total, err := fixture.service.DistinctGenres(ctx, fixture.query())
			if err != nil {
				t.Fatalf("failed to query genres: %v", err)
			}
			if total != 2 {
				t.Errorf("total = %d, want 2", total)
			}

			want := []string{fixture.name("Comedy"), fixture.name("Drama")}
			if got := namesOf(named); !slices.Equal(got, want) {
				t.Errorf("genres = %v, want %v", got, want)
			}

			rows, err := fixture.service.store.Genre.Query().Where(genremodal.NameHasPrefix(fixture.prefix)).Count(ctx)
			if err != nil {
				t.Fatalf("failed to count genre rows: %v", err)
			}
			if rows != 2 {
				t.Errorf("genre rows = %d, want 2", rows)
			}

			credits, err := fixture.service.store.Credit.Query().Where(creditmodal.HasItemWith(itemmodal.ID(movie.ID))).Count(ctx)
			if err != nil {
				t.Fatalf("failed to count credits: %v", err)
			}
			if credits != 2 {
				t.Errorf("credits = %d, want 2", credits)
			}
		})

		t.Run("re-probing drops metadata the container no longer carries", func(t *testing.T) {
			reduced := probe
			reduced.Metadata = ContainerMetadata{Genres: []string{fixture.name("Comedy")}}
			if err := fixture.service.SaveProbe(ctx, movie, reduced); err != nil {
				t.Fatalf("failed to save the probe: %v", err)
			}

			named, _, err := fixture.service.DistinctGenres(ctx, fixture.query())
			if err != nil {
				t.Fatalf("failed to query genres: %v", err)
			}
			if want := []string{fixture.name("Comedy")}; !slices.Equal(namesOf(named), want) {
				t.Errorf("genres = %v, want %v", namesOf(named), want)
			}

			people, _, err := fixture.service.DistinctPeople(ctx, fixture.query(), nil)
			if err != nil {
				t.Fatalf("failed to query people: %v", err)
			}
			if len(people) != 0 {
				t.Errorf("people = %v, want none", namesOf(people))
			}

			tags, err := fixture.service.DistinctTags(ctx, fixture.query())
			if err != nil {
				t.Fatalf("failed to query tags: %v", err)
			}
			if len(tags) != 0 {
				t.Errorf("tags = %v, want none", tags)
			}
		})
	})

	t.Run("two items may share a genre", func(t *testing.T) {
		fixture := newMetadataFixture(t)
		ctx := context.Background()
		shared := fixture.name("Comedy")

		for _, name := range []string{"First", "Second"} {
			probe := Probe{Metadata: ContainerMetadata{Genres: []string{shared}}}
			if err := fixture.service.SaveProbe(ctx, fixture.item(t, name), probe); err != nil {
				t.Fatalf("failed to save the probe for %q: %v", name, err)
			}
		}

		rows, err := fixture.service.store.Genre.Query().Where(genremodal.Name(shared)).Count(ctx)
		if err != nil {
			t.Fatalf("failed to count genre rows: %v", err)
		}
		if rows != 1 {
			t.Errorf("genre rows = %d, want 1", rows)
		}

		named, total, err := fixture.service.DistinctGenres(ctx, fixture.query())
		if err != nil {
			t.Fatalf("failed to query genres: %v", err)
		}
		if got := namesOf(named); !slices.Equal(got, []string{shared}) || total != 1 {
			t.Errorf("genres = %v (%d), want [%s] (1)", got, total, shared)
		}
	})

	t.Run("replaces the streams", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		id := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Movie"})
		item, err := fixture.service.ItemByID(ctx, id)
		if err != nil {
			t.Fatalf("failed to load the item: %v", err)
		}

		first := Probe{
			Container:    "mkv",
			RunTimeTicks: 100,
			Streams: []Stream{
				{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
				{Index: 1, Kind: streammodal.KindAudio, Codec: "aac"},
			},
		}
		if err := fixture.service.SaveProbe(ctx, item, first); err != nil {
			t.Fatalf("failed to save the first probe: %v", err)
		}

		second := Probe{
			Container:    "mkv",
			RunTimeTicks: 200,
			Streams:      []Stream{{Index: 0, Kind: streammodal.KindVideo, Codec: "hevc"}},
		}
		if err := fixture.service.SaveProbe(ctx, item, second); err != nil {
			t.Fatalf("failed to save the second probe: %v", err)
		}

		source, err := fixture.service.MediaSource(ctx, id)
		if err != nil {
			t.Fatalf("failed to query the media source: %v", err)
		}
		if source == nil {
			t.Fatal("media source = nil, want a probed source")
		}
		streams := source.Edges.Streams
		if len(streams) != 1 {
			t.Fatalf("streams = %d, want 1", len(streams))
		}
		if codec := streams[0].Codec; codec != "hevc" {
			t.Errorf("codec = %q, want %q", codec, "hevc")
		}
	})
}

func TestService_DistinctGenres(t *testing.T) {
	fixture := newMetadataFixture(t)
	ctx := context.Background()

	movie := fixture.item(t, "Movie")
	if err := fixture.service.SaveProbe(ctx, movie, Probe{Metadata: ContainerMetadata{Genres: []string{fixture.name("Comedy")}}}); err != nil {
		t.Fatalf("failed to save the probe: %v", err)
	}

	other := uuid.New()

	tests := []struct {
		name  string
		query MetadataQuery
		want  []string
	}{
		{
			name:  "filters by item kind",
			query: MetadataQuery{LibraryID: &fixture.libraryID, Kinds: []Kind{itemmodal.KindEpisode}},
		},
		{
			name:  "filters by search term",
			query: MetadataQuery{LibraryID: &fixture.libraryID, SearchTerm: "comedy"},
			want:  []string{fixture.name("Comedy")},
		},
		{
			name:  "ignores other libraries",
			query: MetadataQuery{LibraryID: &other},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			named, _, err := fixture.service.DistinctGenres(ctx, test.query)
			if err != nil {
				t.Fatalf("failed to query genres: %v", err)
			}
			if got := namesOf(named); !slices.Equal(got, test.want) {
				t.Errorf("genres = %v, want %v", got, test.want)
			}
		})
	}
}
