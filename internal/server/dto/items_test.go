package dto

import (
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestItemDto(t *testing.T) {
	t.Run("hands the locks back", func(t *testing.T) {
		dto := ItemDto(&items.Item{
			Name:         "The Matrix",
			LockData:     true,
			LockedFields: []string{"Name"},
		}, items.Held{}, 0, nil)

		if !apiutil.Deref(dto.LockData) {
			t.Error("lock data = false, want the stored lock: the editor posts back what it was handed")
		}
		want := []api.MetadataField{api.MetadataFieldName}
		if got := apiutil.Deref(dto.LockedFields); !slices.Equal(got, want) {
			t.Errorf("locked fields = %v, want %v: the editor would post an empty set and clear the claim", got, want)
		}
	})

	t.Run("locks nothing by default", func(t *testing.T) {
		dto := ItemDto(&items.Item{Name: "The Matrix"}, items.Held{}, 0, nil)

		if apiutil.Deref(dto.LockData) {
			t.Error("lock data = true, want false")
		}
		if dto.LockedFields != nil {
			t.Errorf("locked fields = %v, want none", apiutil.Deref(dto.LockedFields))
		}
	})

	t.Run("shapes the primary image by what the provider draws", func(t *testing.T) {
		primary := map[string]string{"Primary": "tag"}
		cases := []struct {
			kind items.Kind
			tags map[string]string
			want *float64
		}{
			{kind: itemmodel.KindMovie, tags: primary, want: apiutil.Ptr(items.PosterAspectRatio)},
			{kind: itemmodel.KindSeries, tags: primary, want: apiutil.Ptr(items.PosterAspectRatio)},
			{kind: itemmodel.KindSeason, tags: primary, want: apiutil.Ptr(items.PosterAspectRatio)},
			{kind: itemmodel.KindEpisode, tags: primary, want: apiutil.Ptr(items.StillAspectRatio)},
			{kind: itemmodel.KindMovie, tags: map[string]string{"Backdrop": "tag"}},
			{kind: itemmodel.KindMovie},
		}
		for _, tc := range cases {
			dto := ItemDto(&items.Item{Kind: tc.kind}, items.Held{}, 0, tc.tags)

			got := dto.PrimaryImageAspectRatio
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Errorf("%s with %v: aspect ratio = %v, want %v: jellyfin-web draws a square card without one", tc.kind, tc.tags, apiutil.Deref(got), apiutil.Deref(tc.want))
			}
		}
	})
}
