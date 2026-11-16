package items

import (
	"context"
	"log"
	"slices"

	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const searchedLimit = 10

var requestable = []items.Kind{itemmodel.KindMovie, itemmodel.KindSeries}

func plainSearch(params api.GetItemsParams) bool {
	return apiutil.Deref(params.SearchTerm) != "" &&
		params.ParentId == nil &&
		params.IsFavorite == nil &&
		params.Filters == nil &&
		params.Ids == nil &&
		apiutil.Deref(params.StartIndex) == 0 &&
		apiutil.OrElse(params.Limit, 1) > 0
}

func (s *Server) searched(ctx context.Context, params api.GetItemsParams) ([]api.BaseItemDto, error) {
	if !plainSearch(params) || !s.discover.Enabled() {
		return nil, nil
	}

	asked := dto.Kinds(params.IncludeItemTypes)
	found := []discover.Title{}
	for _, kind := range requestable {
		if asked != nil && !slices.Contains(asked, kind) {
			continue
		}

		taken, err := s.requests.Taken(ctx, kind)
		if err != nil {
			return nil, err
		}
		if !taken {
			continue
		}

		titles, err := s.discover.Search(ctx, kind, *params.SearchTerm, searchedLimit)
		if err != nil {
			log.Printf("items: searching without what the library lacks: %v", err)

			continue
		}
		found = append(found, titles...)
	}

	return dto.UnownedDtos(ctx, s.requests, found)
}
