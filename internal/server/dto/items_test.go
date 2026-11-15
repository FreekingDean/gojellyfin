package dto

import (
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/requests"
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
}

func TestItemDto_PrimaryImageAspectRatio(t *testing.T) {
	for _, test := range []struct {
		kind items.Kind
		tags map[string]string
		want *float64
	}{
		{kind: "Movie", tags: map[string]string{"Primary": "a"}, want: apiutil.Ptr(posterAspect)},
		{kind: "Series", tags: map[string]string{"Primary": "a"}, want: apiutil.Ptr(posterAspect)},
		{kind: "Episode", tags: map[string]string{"Primary": "a"}, want: apiutil.Ptr(stillAspect)},
		{kind: "Movie", tags: map[string]string{"Backdrop": "a"}, want: nil},
	} {
		t.Run(string(test.kind), func(t *testing.T) {
			got := ItemDto(&items.Item{Kind: test.kind}, items.Held{}, 0, test.tags).PrimaryImageAspectRatio
			if (got == nil) != (test.want == nil) || (got != nil && *got != *test.want) {
				t.Errorf("PrimaryImageAspectRatio = %v, want %v", got, test.want)
			}
		})
	}
}

func TestUnownedDto_PrimaryImageAspectRatio(t *testing.T) {
	poster := []items.RemoteImage{{Kind: items.ImageKindPrimary, URL: "https://image.tmdb.org/t/p/w780/poster.jpg"}}

	dto := UnownedDto(discover.Title{Kind: itemmodel.KindMovie, Metadata: items.Metadata{Images: poster}}, requests.Mark{})
	if got := apiutil.Deref(dto.PrimaryImageAspectRatio); got != posterAspect {
		t.Errorf("PrimaryImageAspectRatio = %v, want %v: jellyfin-web draws the detail page's poster square without one", got, posterAspect)
	}

	bare := UnownedDto(discover.Title{Kind: itemmodel.KindMovie}, requests.Mark{})
	if bare.PrimaryImageAspectRatio != nil {
		t.Errorf("PrimaryImageAspectRatio = %v, want none for a title with no poster", *bare.PrimaryImageAspectRatio)
	}
}
