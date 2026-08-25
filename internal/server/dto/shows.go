package dto

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func applyShowFields(ctx context.Context, store *items.Service, records []*items.Item, converted []api.BaseItemDto) error {
	parentIDs := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		if record.ParentID != nil {
			parentIDs = append(parentIDs, *record.ParentID)
		}
	}

	parents, err := store.ItemsByIDs(ctx, parentIDs)
	if err != nil {
		return err
	}

	grandparentIDs := make([]uuid.UUID, 0, len(parents))
	for _, parent := range parents {
		if parent.Kind == itemmodal.KindSeason && parent.ParentID != nil {
			grandparentIDs = append(grandparentIDs, *parent.ParentID)
		}
	}

	grandparents, err := store.ItemsByIDs(ctx, grandparentIDs)
	if err != nil {
		return err
	}

	seriesByIndex := map[int]*items.Item{}
	seriesIDs := make([]uuid.UUID, 0, len(records))
	for index, record := range records {
		if record.ParentID == nil {
			continue
		}

		parent := parents[*record.ParentID]
		if parent == nil {
			continue
		}

		series := parent
		if parent.Kind == itemmodal.KindSeason {
			converted[index].SeasonId = apiutil.Ptr(parent.ID)
			converted[index].SeasonName = apiutil.Ptr(parent.Name)
			if parent.ParentID == nil {
				continue
			}
			series = grandparents[*parent.ParentID]
		}
		if series == nil || series.Kind != itemmodal.KindSeries {
			continue
		}

		converted[index].SeriesId = apiutil.Ptr(series.ID)
		converted[index].SeriesName = apiutil.Ptr(series.Name)
		seriesByIndex[index] = series
		seriesIDs = append(seriesIDs, series.ID)
	}

	tags, err := store.ImageTagsByItem(ctx, seriesIDs)
	if err != nil {
		return err
	}

	for index, series := range seriesByIndex {
		if tag, ok := tags[series.ID]["Primary"]; ok {
			converted[index].SeriesPrimaryImageTag = apiutil.Ptr(tag)
		}
		if tag, ok := tags[series.ID]["Thumb"]; ok {
			converted[index].SeriesThumbImageTag = apiutil.Ptr(tag)
		}
	}

	return nil
}
