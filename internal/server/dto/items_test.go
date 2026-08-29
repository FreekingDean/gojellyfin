package dto

import (
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestItemDto(t *testing.T) {
	t.Run("hands the locks back", func(t *testing.T) {
		dto := ItemDto(&items.Item{
			Name:         "The Matrix",
			LockData:     true,
			LockedFields: []string{"Name"},
		}, "", 0, nil)

		if !apiutil.Deref(dto.LockData) {
			t.Error("lock data = false, want the stored lock: the editor posts back what it was handed")
		}
		want := []api.MetadataField{api.MetadataFieldName}
		if got := apiutil.Deref(dto.LockedFields); !slices.Equal(got, want) {
			t.Errorf("locked fields = %v, want %v: the editor would post an empty set and clear the claim", got, want)
		}
	})

	t.Run("locks nothing by default", func(t *testing.T) {
		dto := ItemDto(&items.Item{Name: "The Matrix"}, "", 0, nil)

		if apiutil.Deref(dto.LockData) {
			t.Error("lock data = true, want false")
		}
		if dto.LockedFields != nil {
			t.Errorf("locked fields = %v, want none", apiutil.Deref(dto.LockedFields))
		}
	})
}
