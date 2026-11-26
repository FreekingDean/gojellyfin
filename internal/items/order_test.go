package items

import (
	"slices"
	"testing"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

var episodicOrderCases = []struct {
	name   string
	sortBy []string
	want   []string
}{
	{name: "no sort key", want: []string{"Pilot", "Zebra Crossing", "Aftermath", "Unnumbered"}},
	{name: "by sort name", sortBy: []string{"SortName"}, want: []string{"Pilot", "Zebra Crossing", "Aftermath", "Unnumbered"}},
	{name: "by name", sortBy: []string{"Name"}, want: []string{"Pilot", "Zebra Crossing", "Aftermath", "Unnumbered"}},
	{name: "by aired order", sortBy: []string{"AiredEpisodeOrder"}, want: []string{"Pilot", "Zebra Crossing", "Aftermath", "Unnumbered"}},
	{name: "by season then episode", sortBy: []string{"ParentIndexNumber", "IndexNumber", "SortName"}, want: []string{"Pilot", "Zebra Crossing", "Aftermath", "Unnumbered"}},
}

func TestQueryItems_EpisodesAreInEpisodicOrder(t *testing.T) {
	fixture := newFixture(t)

	series := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Series"})
	fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Aftermath", sortName: "aftermath", parentID: &series, index: number(1), parentIndex: number(2)})
	fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Zebra Crossing", sortName: "zebra crossing", parentID: &series, index: number(2), parentIndex: number(1)})
	fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Unnumbered", sortName: "unnumbered", parentID: &series})
	fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Pilot", sortName: "pilot", parentID: &series, index: number(1), parentIndex: number(1)})

	for _, test := range episodicOrderCases {
		t.Run(test.name, func(t *testing.T) {
			got := fixture.picked(t, ItemQuery{Kinds: []Kind{itemmodel.KindEpisode}, SortBy: test.sortBy})
			if !slices.Equal(got, test.want) {
				t.Errorf("episodes = %v, want %v", got, test.want)
			}
		})
	}
}

func TestQueryItems_MoviesStillSortByName(t *testing.T) {
	fixture := newFixture(t)

	for _, name := range []string{"Blade Runner", "Alien", "Casablanca"} {
		fixture.add(t, seed{kind: itemmodel.KindMovie, name: name, sortName: SortName(name)})
	}

	got := fixture.picked(t, ItemQuery{SortBy: []string{"SortName"}})
	if want := []string{"Alien", "Blade Runner", "Casablanca"}; !slices.Equal(got, want) {
		t.Errorf("movies = %v, want %v", got, want)
	}
}
