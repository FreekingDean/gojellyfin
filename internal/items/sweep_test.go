package items

import (
	"context"
	"errors"
	"testing"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestDeleteItemsNotInKeysRefusesAnEmptyScan(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	kept := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Kept"})

	err := fixture.service.DeleteItemsNotInKeys(ctx, fixture.libraryID, nil)
	if !errors.Is(err, ErrNothingScanned) {
		t.Fatalf("got %v, want ErrNothingScanned", err)
	}

	if _, err := fixture.service.ItemByID(ctx, kept); err != nil {
		t.Errorf("an empty scan deleted the item: %v", err)
	}
}
