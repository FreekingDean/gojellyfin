package mediasegment

import (
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/segments"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func segmentDto(itemID uuid.UUID, segment *segments.Segment) api.MediaSegmentDto {
	return api.MediaSegmentDto{
		Id:         apiutil.Ptr(segment.ID),
		ItemId:     apiutil.Ptr(itemID),
		Type:       apiutil.Ptr(api.MediaSegmentType(segment.Kind)),
		StartTicks: apiutil.Ptr(segment.StartTicks),
		EndTicks:   apiutil.Ptr(segment.EndTicks),
	}
}
