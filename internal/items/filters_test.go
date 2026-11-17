package items

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (f *fixture) viewer(t *testing.T) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	name := t.Name() + "-" + uuid.NewString()
	user, err := f.client.User.Create().SetName(name).SetUsername(name).SetPasswordHash(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}
	t.Cleanup(func() {
		if err := f.client.User.DeleteOne(user).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
	})

	return user.ID
}

func TestQueryItems_UserDataFilters(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	user, other := fixture.viewer(t), fixture.viewer(t)

	watched := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Watched", sortName: "watched"})
	loved := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Loved", sortName: "loved"})
	halfway := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Halfway", sortName: "halfway"})
	fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Untouched", sortName: "untouched"})
	fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Show", sortName: "show"})

	for _, datum := range []*Datum{
		{UserID: user, ItemID: watched, Played: true, PlayCount: 1},
		{UserID: user, ItemID: loved, IsFavorite: true},
		{UserID: user, ItemID: halfway, PlaybackPositionTicks: 600_000_000},
		{UserID: other, ItemID: halfway, Played: true, IsFavorite: true},
	} {
		if err := fixture.service.SaveUserItemDatum(ctx, datum); err != nil {
			t.Fatalf("failed to save user data: %v", err)
		}
	}

	yes, no := true, false
	for _, test := range []queryItemsUserDataFiltersCase{
		{
			name:  "played takes what this user finished",
			query: ItemQuery{User: user, Played: &yes},
			want:  []string{"Watched"},
		},
		{
			name:  "unplayed takes everything else, untouched rows included",
			query: ItemQuery{User: user, Played: &no},
			want:  []string{"Halfway", "Loved", "Show", "Untouched"},
		},
		{
			name:  "favourite takes this user's hearts and not another's",
			query: ItemQuery{User: user, Favorite: &yes},
			want:  []string{"Loved"},
		},
		{
			name:  "resumable takes what this user stopped part way",
			query: ItemQuery{User: user, Resumable: true},
			want:  []string{"Halfway"},
		},
		{
			name:  "folder takes series and seasons",
			query: ItemQuery{User: user, Folder: &yes},
			want:  []string{"Show"},
		},
		{
			name:  "not folder takes what plays",
			query: ItemQuery{User: user, Folder: &no},
			want:  []string{"Halfway", "Loved", "Untouched", "Watched"},
		},
		{
			name:  "an excluded kind drops out",
			query: ItemQuery{User: user, ExcludeKinds: []Kind{itemmodel.KindMovie}},
			want:  []string{"Show"},
		},
		{
			name:  "the search page's videos row finds no film a second time",
			query: ItemQuery{User: user, MediaTypes: []MediaType{"Video"}, ExcludeKinds: []Kind{itemmodel.KindMovie, itemmodel.KindEpisode}},
			want:  nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := fixture.picked(t, test.query); !slices.Equal(got, test.want) {
				t.Errorf("items = %v, want %v", got, test.want)
			}
		})
	}
}

type queryItemsUserDataFiltersCase struct {
	name  string
	query ItemQuery
	want  []string
}
