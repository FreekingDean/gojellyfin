package items

import (
	"context"
	"testing"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestQueryItems_Limit(t *testing.T) {
	fixture := newFixture(t)

	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		fixture.add(t, seed{kind: itemmodel.KindEpisode, name: name, sortName: SortName(name)})
	}

	none, one := 0, 1
	for _, test := range []struct {
		name  string
		limit *int
		want  int
	}{
		{name: "no limit answers every row", limit: nil, want: 3},
		{name: "a limit of zero answers only the count", limit: &none, want: 0},
		{name: "a positive limit pages", limit: &one, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			records, total, err := fixture.service.QueryItems(context.Background(), ItemQuery{
				Viewer:    Everyone,
				LibraryID: &fixture.libraryID,
				Kinds:     []Kind{itemmodel.KindEpisode},
				Limit:     test.limit,
			})
			if err != nil {
				t.Fatalf("failed to query items: %v", err)
			}
			if len(records) != test.want {
				t.Errorf("records = %d, want %d", len(records), test.want)
			}
			if total != 3 {
				t.Errorf("total = %d, want 3", total)
			}
		})
	}
}
