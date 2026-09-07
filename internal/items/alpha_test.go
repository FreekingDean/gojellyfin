package items

import (
	"context"
	"slices"
	"testing"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (f *fixture) picked(t *testing.T, query ItemQuery) []string {
	t.Helper()

	query.Viewer = Everyone
	query.LibraryID = &f.libraryID

	records, _, err := f.service.QueryItems(context.Background(), query)
	if err != nil {
		t.Fatalf("failed to query items: %v", err)
	}

	found := make([]string, 0, len(records))
	for _, record := range records {
		found = append(found, record.Name)
	}

	return found
}

func TestQueryItems_AlphaPicker(t *testing.T) {
	fixture := newFixture(t)

	for _, name := range []string{"Alien", "Arrival", "Blade Runner", "The Matrix", "2001"} {
		fixture.add(t, seed{kind: itemmodal.KindMovie, name: name, sortName: SortName(name)})
	}

	for _, test := range []struct {
		name  string
		query ItemQuery
		want  []string
	}{
		{
			name:  "a letter takes the titles under it",
			query: ItemQuery{NameStartsWith: "A"},
			want:  []string{"Alien", "Arrival"},
		},
		{
			name:  "the letter is matched however the client cased it",
			query: ItemQuery{NameStartsWith: "b"},
			want:  []string{"Blade Runner"},
		},
		{
			name:  "an article is stripped, so The Matrix is under M",
			query: ItemQuery{NameStartsWith: "M"},
			want:  []string{"The Matrix"},
		},
		{
			name:  "nothing sorts under the article itself",
			query: ItemQuery{NameStartsWith: "T"},
			want:  []string{},
		},
		{
			name:  "hash takes everything sorting before the letters",
			query: ItemQuery{NameLessThan: "A"},
			want:  []string{"2001"},
		},
		{
			name:  "or greater takes the tail of the alphabet",
			query: ItemQuery{NameStartsWithOrGreater: "M"},
			want:  []string{"The Matrix"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := fixture.picked(t, test.query); !slices.Equal(got, test.want) {
				t.Errorf("items = %v, want %v", got, test.want)
			}
		})
	}
}
