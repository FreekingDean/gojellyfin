package items

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	creditmodel "github.com/FreekingDean/gojellyfin/internal/store/credit"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_Similar(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	drama, heist, crew := "Drama "+uuid.NewString(), "Heist "+uuid.NewString(), "Person "+uuid.NewString()

	describe := func(id uuid.UUID, genres []string, people ...string) {
		t.Helper()

		credits := make([]Credit, 0, len(people))
		for _, name := range people {
			credits = append(credits, Credit{Name: name, Kind: creditmodel.KindActor})
		}
		if err := fixture.service.replaceNamed(ctx, id, Metadata{Genres: &genres, People: &credits}); err != nil {
			t.Fatalf("failed to describe %s: %v", id, err)
		}
	}

	origin := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Seed"})
	closest := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Closest"})
	near := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Near"})
	unrelated := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Unrelated"})
	show := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Show"})
	season := fixture.add(t, seed{kind: itemmodel.KindSeason, name: "Season", parentID: &show})
	other := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Other Show"})

	describe(origin, []string{drama, heist}, crew)
	describe(closest, []string{drama, heist})
	describe(near, []string{drama})
	describe(unrelated, []string{"Comedy " + uuid.NewString()})
	describe(show, []string{drama, heist}, crew)
	describe(other, []string{drama})

	viewer := Viewer{Libraries: []uuid.UUID{fixture.libraryID}}
	found, err := fixture.service.Similar(ctx, viewer, origin, 10)
	if err != nil {
		t.Fatalf("failed to rank: %v", err)
	}
	names := make([]string, 0, len(found))
	for _, record := range found {
		names = append(names, record.Name)
	}
	if len(names) != 2 || names[0] != "Closest" || names[1] != "Near" {
		t.Errorf("similar = %v, want Closest then Near, of the seed's kind and never the seed", names)
	}

	fromSeason, err := fixture.service.Similar(ctx, viewer, season, 10)
	if err != nil {
		t.Fatalf("failed to rank from a season: %v", err)
	}
	if len(fromSeason) != 1 || fromSeason[0].ID != other {
		t.Errorf("from a season = %v, want the series' peer", fromSeason)
	}

	if _, err := fixture.service.Similar(ctx, viewer, uuid.New(), 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound for an id nobody holds", err)
	}
}
