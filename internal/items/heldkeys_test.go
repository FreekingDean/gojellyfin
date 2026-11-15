package items

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_HeldKeys(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t)

	held := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Held"})
	fixture.source(t, held, "/"+fixture.libraryID.String()+"/held.mkv")
	fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Fileless"})
	gone := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Gone"})
	if err := fixture.client.Item.UpdateOneID(gone).SetDeletedAt(time.Now()).Exec(ctx); err != nil {
		t.Fatalf("failed to delete the item: %v", err)
	}

	key := func(name string) string { return fmt.Sprintf("test:%s:%s", fixture.libraryID, name) }

	tests := []struct {
		name string
		keys []string
		want []string
	}{
		{name: "a title the library holds", keys: []string{key("Held")}, want: []string{key("Held")}},
		{name: "a title with no file is not held", keys: []string{key("Fileless")}, want: []string{}},
		{name: "a title that was swept is not held", keys: []string{key("Gone")}, want: []string{}},
		{name: "a title nothing ever reported", keys: []string{key("Never")}, want: []string{}},
		{name: "only the held of several", keys: []string{key("Never"), key("Held"), key("Gone")}, want: []string{key("Held")}},
		{name: "no keys asks nothing", keys: nil, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			held, err := fixture.service.HeldKeys(ctx, test.keys)
			if err != nil {
				t.Fatalf("failed to query the keys: %v", err)
			}
			if !slices.Equal(held, test.want) {
				t.Errorf("held = %v, want %v", held, test.want)
			}
		})
	}
}
