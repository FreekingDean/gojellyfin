package dto

import (
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestFilter(t *testing.T) {
	user := uuid.New()

	for _, test := range []struct {
		name   string
		params api.GetItemsParams
		want   items.ItemQuery
	}{
		{
			name:   "the favourites tab asks by filter",
			params: api.GetItemsParams{Filters: &[]api.ItemFilter{api.ItemFilterIsFavorite}},
			want:   items.ItemQuery{User: user, Favorite: apiutil.Ptr(true)},
		},
		{
			name:   "unplayed and resumable ride together",
			params: api.GetItemsParams{Filters: &[]api.ItemFilter{api.ItemFilterIsUnplayed, api.ItemFilterIsResumable}},
			want:   items.ItemQuery{User: user, Played: apiutil.Ptr(false), Resumable: true},
		},
		{
			name:   "the boolean parameters are read too",
			params: api.GetItemsParams{IsPlayed: apiutil.Ptr(true), IsFavorite: apiutil.Ptr(false)},
			want:   items.ItemQuery{User: user, Played: apiutil.Ptr(true), Favorite: apiutil.Ptr(false)},
		},
		{
			name:   "not folder narrows to what plays",
			params: api.GetItemsParams{Filters: &[]api.ItemFilter{api.ItemFilterIsNotFolder}},
			want:   items.ItemQuery{User: user, Folder: apiutil.Ptr(false)},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got items.ItemQuery
			filter(&got, user, test.params)

			if got.User != test.want.User ||
				apiutil.Deref(got.Played) != apiutil.Deref(test.want.Played) || (got.Played == nil) != (test.want.Played == nil) ||
				apiutil.Deref(got.Favorite) != apiutil.Deref(test.want.Favorite) || (got.Favorite == nil) != (test.want.Favorite == nil) ||
				apiutil.Deref(got.Folder) != apiutil.Deref(test.want.Folder) || (got.Folder == nil) != (test.want.Folder == nil) ||
				got.Resumable != test.want.Resumable {
				t.Errorf("query = %+v, want %+v", got, test.want)
			}
		})
	}
}
