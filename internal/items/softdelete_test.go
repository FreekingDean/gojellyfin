package items

import (
	"context"
	"testing"
	"time"

	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestService_SaveScanned(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	first, err := fixture.service.SaveScanned(ctx, Scanned{
		LibraryID:    fixture.libraryID,
		Kind:         itemmodal.KindMovie,
		Name:         "Returns",
		SortName:     "Returns",
		Path:         "/media/returns.mkv",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	if err := fixture.service.DeleteItemsNotInPaths(ctx, fixture.libraryID, []string{"/somewhere/else.mkv"}); err != nil {
		t.Fatalf("failed to sweep: %v", err)
	}

	second, err := fixture.service.SaveScanned(ctx, Scanned{
		LibraryID:    fixture.libraryID,
		Kind:         itemmodal.KindMovie,
		Name:         "Returns",
		SortName:     "Returns",
		Path:         "/media/returns.mkv",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the returning item: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("the file came back as a new item: %s, was %s", second.ID, first.ID)
	}
	if second.DeletedAt != nil {
		t.Error("the returning item is still marked deleted")
	}
	if _, err := fixture.service.ItemByID(ctx, first.ID); err != nil {
		t.Errorf("the revived item is not readable: %v", err)
	}
}
