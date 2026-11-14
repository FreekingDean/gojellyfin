package items

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (f *fixture) added(t *testing.T, id uuid.UUID, at time.Time) {
	t.Helper()

	if err := f.client.Item.UpdateOneID(id).SetDateModified(at).Exec(context.Background()); err != nil {
		t.Fatalf("failed to date the item: %v", err)
	}
}

func latestNames(latest []Latest) []string {
	var found []string
	for _, entry := range latest {
		found = append(found, entry.Item.Name)
	}

	return found
}

func latestCounts(latest []Latest) []int32 {
	var found []int32
	for _, entry := range latest {
		found = append(found, entry.Count)
	}

	return found
}

func TestService_LatestItems(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	now := time.Date(2026, time.November, 14, 0, 0, 0, 0, time.UTC)
	day := func(days int) *time.Time {
		at := now.AddDate(0, 0, -days)
		return &at
	}

	ended := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Ended"})
	endedSeason := fixture.add(t, seed{kind: itemmodel.KindSeason, name: "Ended S1", parentID: &ended, index: number(1)})
	endedOne := fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Ended E1", parentID: &endedSeason, index: number(1), parentIndex: number(1), premiere: day(3000)})
	endedTwo := fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Ended E2", parentID: &endedSeason, index: number(2), parentIndex: number(1), premiere: day(2990)})

	airing := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Airing"})
	airingSeason := fixture.add(t, seed{kind: itemmodel.KindSeason, name: "Airing S1", parentID: &airing, index: number(1)})
	airingOne := fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Airing E1", parentID: &airingSeason, index: number(1), parentIndex: number(1), premiere: day(2)})
	unaired := fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Airing E2", parentID: &airingSeason, index: number(2), parentIndex: number(1), premiere: func() *time.Time { at := now.AddDate(0, 0, 5); return &at }()})

	movie := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Movie", premiere: day(30)})

	fixture.added(t, endedOne, *day(0))
	fixture.added(t, endedTwo, *day(0))
	fixture.added(t, airingOne, *day(2))
	fixture.added(t, unaired, *day(1))
	fixture.added(t, movie, *day(10))

	tests := []struct {
		name       string
		query      LatestQuery
		wantNames  []string
		wantCounts []int32
	}{
		{
			name:       "added orders by the file's date and groups a series holding several",
			query:      LatestQuery{Viewer: Everyone, Order: LatestAdded},
			wantNames:  []string{"Ended", "Airing", "Movie"},
			wantCounts: []int32{2, 2, 0},
		},
		{
			name:       "aired orders by premiere, leaves out what has not aired and keeps a lone episode as itself",
			query:      LatestQuery{Viewer: Everyone, Order: LatestAired},
			wantNames:  []string{"Airing E1", "Movie", "Ended"},
			wantCounts: []int32{0, 0, 2},
		},
		{
			name:       "ungrouped answers every episode",
			query:      LatestQuery{Viewer: Everyone, Order: LatestAired, Ungrouped: true},
			wantNames:  []string{"Airing E1", "Movie", "Ended E2", "Ended E1"},
			wantCounts: []int32{0, 0, 0, 0},
		},
		{
			name:       "the limit counts groups rather than episodes",
			query:      LatestQuery{Viewer: Everyone, Order: LatestAdded, Limit: 1},
			wantNames:  []string{"Ended"},
			wantCounts: []int32{2},
		},
		{
			name:  "the zero viewer sees nothing",
			query: LatestQuery{Order: LatestAdded, Viewer: Viewer{}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := test.query
			query.LibraryID = &fixture.libraryID
			query.Now = now

			latest, err := fixture.service.LatestItems(ctx, query)
			if err != nil {
				t.Fatalf("failed to query the latest items: %v", err)
			}

			if got := latestNames(latest); !slices.Equal(got, test.wantNames) {
				t.Errorf("names = %v, want %v", got, test.wantNames)
			}
			if got := latestCounts(latest); !slices.Equal(got, test.wantCounts) {
				t.Errorf("counts = %v, want %v", got, test.wantCounts)
			}
		})
	}
}
