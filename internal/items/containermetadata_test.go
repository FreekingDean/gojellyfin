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
	record, err := f.service.ItemByID(context.Background(), Everyone, id)
	if err != nil {
		t.Fatalf("failed to read %q: %v", name, err)
	}

	return record
}

type seeded struct {
	Genres  []string
	Studios []string
	Tags    []string
	People  []Credit
}

func (f *metadataFixture) seed(t *testing.T, item *Item, with seeded) {
	t.Helper()

	ctx := context.Background()

	metadata := Metadata{}
	if with.Genres != nil {
		metadata.Genres = &with.Genres
	}
	if with.Studios != nil {
		metadata.Studios = &with.Studios
	}
	if with.Tags != nil {
		metadata.Tags = &with.Tags
	}
	if with.People != nil {
		metadata.People = &with.People
	}
	if _, err := f.service.UpdateMetadata(ctx, item.ID, metadata); err != nil {
		t.Fatalf("failed to save the metadata: %v", err)
	}
}

func (f *metadataFixture) query() MetadataQuery {
	return MetadataQuery{Viewer: Everyone, LibraryID: &f.libraryID}
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

func TestService_NamedMetadata(t *testing.T) {
	t.Run("writes the container metadata", func(t *testing.T) {
		fixture := newMetadataFixture(t)
		ctx := context.Background()
		movie := fixture.item(t, "Movie")

		fixture.seed(t, movie, seeded{
			Genres:  []string{fixture.name("Comedy"), fixture.name("Drama")},
			Studios: []string{fixture.name("Studio")},
			Tags:    []string{"live"},
			People: []Credit{
				{Name: fixture.name("Director"), Kind: creditmodal.KindDirector},
				{Name: fixture.name("Writer"), Kind: creditmodal.KindWriter, Role: "Teleplay", Order: 3},
			},
		})

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

		t.Run("takes a person the provider credited twice", func(t *testing.T) {
			twice := newMetadataFixture(t)
			movie := twice.item(t, "Twice")

			twice.seed(t, movie, seeded{People: []Credit{
				{Name: twice.name("Producer"), Kind: creditmodal.KindProducer},
				{Name: twice.name("Producer"), Kind: creditmodal.KindProducer},
			}})

			named, _, err := twice.service.DistinctPeople(ctx, MetadataQuery{
				Viewer: Everyone,
				ItemID: &movie.ID,
			}, nil)
			if err != nil {
				t.Fatalf("failed to query people: %v", err)
			}

			want := []string{twice.name("Producer")}
			if got := namesOf(named); !slices.Equal(got, want) {
				t.Errorf("people = %v, want %v", got, want)
			}
		})

		t.Run("keeps the role and the billing order", func(t *testing.T) {
			credit, err := fixture.service.store.Credit.Query().
				Where(creditmodal.HasPersonWith(personmodal.Name(fixture.name("Writer")))).
				Only(ctx)
			if err != nil {
				t.Fatalf("failed to read the credit: %v", err)
			}

			if credit.Role != "Teleplay" || credit.SortOrder != 3 {
				t.Errorf("credit = %q/%d, want Teleplay/3", credit.Role, credit.SortOrder)
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

		t.Run("writing the same metadata again changes nothing", func(t *testing.T) {
			fixture.seed(t, movie, seeded{
				Genres:  []string{fixture.name("Comedy"), fixture.name("Drama")},
				Studios: []string{fixture.name("Studio")},
			})

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

		})

		t.Run("a shorter list replaces the one it names and leaves the rest", func(t *testing.T) {
			fixture.seed(t, movie, seeded{Genres: []string{fixture.name("Comedy")}})

			named, _, err := fixture.service.DistinctGenres(ctx, fixture.query())
			if err != nil {
				t.Fatalf("failed to query genres: %v", err)
			}
			if want := []string{fixture.name("Comedy")}; !slices.Equal(namesOf(named), want) {
				t.Errorf("genres = %v, want %v", namesOf(named), want)
			}

			studios, _, err := fixture.service.DistinctStudios(ctx, fixture.query())
			if err != nil {
				t.Fatalf("failed to query studios: %v", err)
			}
			if want := []string{fixture.name("Studio")}; !slices.Equal(namesOf(studios), want) {
				t.Errorf("studios = %v, want them untouched by a write that named none", namesOf(studios))
			}
		})
	})

	t.Run("two items may share a genre", func(t *testing.T) {
		fixture := newMetadataFixture(t)
		ctx := context.Background()
		shared := fixture.name("Comedy")

		for _, name := range []string{"First", "Second"} {
			fixture.seed(t, fixture.item(t, name), seeded{Genres: []string{shared}})
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
		item, err := fixture.service.ItemByID(ctx, Everyone, id)
		if err != nil {
			t.Fatalf("failed to load the item: %v", err)
		}

		first := MediaSource{
			Container:    "mkv",
			RunTimeTicks: 100,
			Edges: MediaSourceEdges{Streams: []*MediaStream{
				{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
				{Index: 1, Kind: streammodal.KindAudio, Codec: "aac"},
			}},
		}
		source := fixture.source(t, id, "/media/movie.mkv")
		if err := fixture.service.SaveProbe(ctx, item, source, first); err != nil {
			t.Fatalf("failed to save the first probe: %v", err)
		}

		second := MediaSource{
			Container:    "mkv",
			RunTimeTicks: 200,
			Edges:        MediaSourceEdges{Streams: []*MediaStream{{Index: 0, Kind: streammodal.KindVideo, Codec: "hevc"}}},
		}
		if err := fixture.service.SaveProbe(ctx, item, source, second); err != nil {
			t.Fatalf("failed to save the second probe: %v", err)
		}

		probed, err := fixture.service.MediaSources(ctx, id)
		if err != nil {
			t.Fatalf("failed to query the media sources: %v", err)
		}
		if len(probed) != 1 {
			t.Fatalf("media sources = %d, want the one probed source", len(probed))
		}
		streams := probed[0].Edges.Streams
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
	fixture.seed(t, movie, seeded{Genres: []string{fixture.name("Comedy")}})

	t.Run("filters by item kind", func(t *testing.T) {
		named, _, err := fixture.service.DistinctGenres(ctx, MetadataQuery{Viewer: Everyone,
			LibraryID: &fixture.libraryID,
			Kinds:     []Kind{itemmodal.KindEpisode},
		})
		if err != nil {
			t.Fatalf("failed to query genres: %v", err)
		}
		if len(named) != 0 {
			t.Errorf("genres = %v, want none", namesOf(named))
		}
	})

	t.Run("filters by search term", func(t *testing.T) {
		named, _, err := fixture.service.DistinctGenres(ctx, MetadataQuery{Viewer: Everyone,
			LibraryID:  &fixture.libraryID,
			SearchTerm: "comedy",
		})
		if err != nil {
			t.Fatalf("failed to query genres: %v", err)
		}
		if want := []string{fixture.name("Comedy")}; !slices.Equal(namesOf(named), want) {
			t.Errorf("genres = %v, want %v", namesOf(named), want)
		}
	})

	t.Run("ignores other libraries", func(t *testing.T) {
		other := uuid.New()
		named, _, err := fixture.service.DistinctGenres(ctx, MetadataQuery{Viewer: Everyone, LibraryID: &other})
		if err != nil {
			t.Fatalf("failed to query genres: %v", err)
		}
		if len(named) != 0 {
			t.Errorf("genres = %v, want none", namesOf(named))
		}
	})
}
