package metadata

import (
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

func TestStripLockedFields_KeepsAnEditorsEdges(t *testing.T) {
	answered := items.Metadata{
		Genres:  &[]string{"Drama"},
		Studios: &[]string{"Example Pictures"},
		People:  &[]items.Credit{{Name: "Ada Example"}},
	}

	stripLockedFields(&answered, []string{items.LockedGenres, items.LockedStudios, items.LockedCast})

	if answered.Genres != nil || answered.Studios != nil || answered.People != nil {
		t.Errorf("metadata = %+v, want the locked edges left out", answered)
	}
}
