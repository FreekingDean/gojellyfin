package items

import (
	"context"
	"testing"

	"github.com/google/uuid"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_Ancestors(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t)

	series := fixture.add(t, seed{kind: itemmodel.KindSeries, name: "Series"})
	season := fixture.add(t, seed{kind: itemmodel.KindSeason, name: "Season", parentID: &series})
	episode := fixture.add(t, seed{kind: itemmodel.KindEpisode, name: "Episode", parentID: &season})

	ancestry, err := fixture.service.Ancestors(ctx, episode)
	if err != nil {
		t.Fatalf("Ancestors: %v", err)
	}
	if len(ancestry.Parents) != 2 || ancestry.Parents[0].ID != season || ancestry.Parents[1].ID != series {
		t.Errorf("parents = %v, want the season then the series", ancestry.Parents)
	}
	if len(ancestry.LibraryIDs) != 1 || ancestry.LibraryIDs[0] != fixture.libraryID {
		t.Errorf("libraries = %v, want %s", ancestry.LibraryIDs, fixture.libraryID)
	}

	top, err := fixture.service.Ancestors(ctx, series)
	if err != nil {
		t.Fatalf("Ancestors: %v", err)
	}
	if len(top.Parents) != 0 {
		t.Errorf("a series has parents %v, want none", top.Parents)
	}

	if err := fixture.service.store.Item.UpdateOneID(series).SetParentID(episode).Exec(ctx); err != nil {
		t.Fatalf("failed to loop the chain: %v", err)
	}
	looped, err := fixture.service.Ancestors(ctx, episode)
	if err != nil {
		t.Fatalf("Ancestors over a cycle: %v", err)
	}
	if len(looped.Parents) != 2 {
		t.Errorf("a cycle answered %d parents, want 2", len(looped.Parents))
	}

	missing, err := fixture.service.Ancestors(ctx, uuid.New())
	if err != nil || missing != nil {
		t.Errorf("an unknown id answered %v, %v, want nothing", missing, err)
	}
}
