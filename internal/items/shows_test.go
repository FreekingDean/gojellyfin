package items

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type fixture struct {
	service   *Service
	libraryID uuid.UUID
}

type seed struct {
	kind        Kind
	name        string
	sortName    string
	parentID    *uuid.UUID
	index       *int32
	parentIndex *int32
	premiere    *time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{service: New(client), libraryID: library.ID}
}

func (f *fixture) add(t *testing.T, item seed) uuid.UUID {
	t.Helper()

	sortName := item.sortName
	if sortName == "" {
		sortName = item.name
	}

	record, err := f.service.store.Item.Create().
		SetLibraryID(f.libraryID).
		SetKind(item.kind).
		SetName(item.name).
		SetSortName(sortName).
		SetKey(fmt.Sprintf("test:%s", item.name)).
		SetNillableParentID(item.parentID).
		SetNillableIndexNumber(item.index).
		SetNillableParentIndexNumber(item.parentIndex).
		SetNillablePremiereDate(item.premiere).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", item.name, err)
	}

	return record.ID
}

func (f *fixture) source(t *testing.T, itemID uuid.UUID, path string) *MediaSource {
	t.Helper()

	source, err := f.service.SaveSource(context.Background(), ScannedSource{
		LibraryID: f.libraryID,
		ItemID:    itemID,
		Path:      path,
		Name:      path,
	})
	if err != nil {
		t.Fatalf("failed to create the media source: %v", err)
	}

	return source
}

func number(value int32) *int32 {
	return &value
}

func names(records []*Item) []string {
	found := make([]string, 0, len(records))
	for _, record := range records {
		found = append(found, record.Name)
	}

	return found
}

func TestService_SeriesSeasons(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Series"})
	other := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Other Series"})

	fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 2", parentID: &series, index: number(2)})
	fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series, index: number(1)})
	fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Unnumbered B", parentID: &series, sortName: "b"})
	fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Unnumbered A", parentID: &series, sortName: "a"})
	fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Elsewhere", parentID: &other, index: number(1)})
	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "Loose Episode", parentID: &series, index: number(1)})

	records, err := fixture.service.SeriesSeasons(ctx, series)
	if err != nil {
		t.Fatalf("failed to query seasons: %v", err)
	}

	want := []string{"Season 1", "Season 2", "Unnumbered A", "Unnumbered B"}
	if got := names(records); !slices.Equal(got, want) {
		t.Errorf("seasons = %v, want %v", got, want)
	}
}

func TestService_SeriesEpisodes(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Series"})
	other := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Other Series"})
	seasonOne := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series, index: number(1)})
	seasonTwo := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 2", parentID: &series, index: number(2)})
	otherSeason := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Other Season", parentID: &other, index: number(1)})

	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S02E01", parentID: &seasonTwo, index: number(1), parentIndex: number(2)})
	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S01E02", parentID: &seasonOne, index: number(2), parentIndex: number(1)})
	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S01E01", parentID: &seasonOne, index: number(1), parentIndex: number(1)})
	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S02 Extra", parentID: &seasonTwo, parentIndex: number(2)})
	fixture.add(t, seed{kind: itemmodal.KindVideo, name: "Behind The Scenes", parentID: &seasonOne, index: number(1), parentIndex: number(1)})
	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "Elsewhere", parentID: &otherSeason, index: number(1), parentIndex: number(1)})

	tests := []struct {
		name      string
		query     EpisodeQuery
		want      []string
		wantTotal int
	}{
		{
			name:      "orders by season then episode",
			query:     EpisodeQuery{SeriesID: series},
			want:      []string{"S01E01", "S01E02", "S02E01", "S02 Extra"},
			wantTotal: 4,
		},
		{
			name:      "filters by season id",
			query:     EpisodeQuery{SeriesID: series, SeasonID: &seasonTwo},
			want:      []string{"S02E01", "S02 Extra"},
			wantTotal: 2,
		},
		{
			name:      "filters by season number",
			query:     EpisodeQuery{SeriesID: series, Season: number(1)},
			want:      []string{"S01E01", "S01E02"},
			wantTotal: 2,
		},
		{
			name:      "pages without changing the total",
			query:     EpisodeQuery{SeriesID: series, StartIndex: 1, Limit: 2},
			want:      []string{"S01E02", "S02E01"},
			wantTotal: 4,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			records, total, err := fixture.service.SeriesEpisodes(ctx, test.query)
			if err != nil {
				t.Fatalf("failed to query episodes: %v", err)
			}

			if got := names(records); !slices.Equal(got, test.want) {
				t.Errorf("episodes = %v, want %v", got, test.want)
			}
			if total != test.wantTotal {
				t.Errorf("total = %d, want %d", total, test.wantTotal)
			}
		})
	}
}

func TestService_UpcomingEpisodes(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	now := time.Now()
	aired := now.Add(-24 * time.Hour)
	soon := now.Add(24 * time.Hour)
	later := now.Add(48 * time.Hour)

	series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Series"})
	season := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series, index: number(1)})

	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "Later", parentID: &season, index: number(3), premiere: &later})
	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "Soon", parentID: &season, index: number(2), premiere: &soon})
	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "Aired", parentID: &season, index: number(1), premiere: &aired})
	fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "Undated", parentID: &season, index: number(4)})

	tests := []struct {
		name       string
		startIndex int
		limit      int
		want       []string
		wantTotal  int
	}{
		{
			name:      "returns only unaired episodes",
			want:      []string{"Soon", "Later"},
			wantTotal: 2,
		},
		{
			name:       "pages without changing the total",
			startIndex: 1,
			limit:      1,
			want:       []string{"Later"},
			wantTotal:  2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			records, total, err := fixture.service.UpcomingEpisodes(ctx, &fixture.libraryID, test.startIndex, test.limit)
			if err != nil {
				t.Fatalf("failed to query upcoming episodes: %v", err)
			}

			if got := names(records); !slices.Equal(got, test.want) {
				t.Errorf("upcoming = %v, want %v", got, test.want)
			}
			if total != test.wantTotal {
				t.Errorf("total = %d, want %d", total, test.wantTotal)
			}
		})
	}
}
