package mediasegment

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/segments"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items    *items.Service
	segments *segments.Service
	policies dto.Access
}

func New(records *items.Service, found *segments.Service, policies dto.Access) *Server {
	return &Server{items: records, segments: found, policies: policies}
}

func (s *Server) GetItemSegments(ctx context.Context, request api.GetItemSegmentsRequestObject) (api.GetItemSegmentsResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	held, err := s.items.ItemsByIDs(ctx, viewer, []uuid.UUID{request.ItemId})
	if err != nil {
		return nil, err
	}
	if _, ok := held[request.ItemId]; !ok {
		return api.GetItemSegments404JSONResponse{}, nil
	}

	var kinds []segments.Kind
	for _, kind := range apiutil.Deref(request.Params.IncludeSegmentTypes) {
		kinds = append(kinds, segments.Kind(kind))
	}

	found, err := s.segments.ForItem(ctx, request.ItemId, kinds)
	if err != nil {
		return nil, err
	}

	answered := make([]api.MediaSegmentDto, 0, len(found))
	for _, segment := range found {
		answered = append(answered, segmentDto(request.ItemId, segment))
	}

	return api.GetItemSegments200JSONResponse{
		Items:            &answered,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(answered))),
	}, nil
}
