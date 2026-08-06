package userlibrary

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
)

func (s *Server) GetResumeItems(ctx context.Context, request api.GetResumeItemsRequestObject) (api.GetResumeItemsResponseObject, error) {
	// Everything scanned so far is video; asking for Book or Audio can only
	// ever be empty.
	if !wantsVideo(request.Params.MediaTypes) {
		return api.GetResumeItems200JSONResponse(emptyResult()), nil
	}

	records, total, err := s.items.ResumeItems(ctx,
		auth.UserID(ctx),
		itemTypes(request.Params.IncludeItemTypes),
		request.Params.ParentId,
		int(dtos.Deref(request.Params.StartIndex)),
		int(dtos.Deref(request.Params.Limit)),
	)
	if err != nil {
		return nil, err
	}

	items, err := dtos.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetResumeItems200JSONResponse{
		Items:            &items,
		StartIndex:       dtos.Ptr(int32(dtos.Deref(request.Params.StartIndex))),
		TotalRecordCount: dtos.Ptr(int32(total)),
	}, nil
}

func wantsVideo(mediaTypes *[]api.MediaType) bool {
	if mediaTypes == nil || len(*mediaTypes) == 0 {
		return true
	}

	for _, mediaType := range *mediaTypes {
		if mediaType == api.MediaTypeVideo {
			return true
		}
	}

	return false
}

func itemTypes(kinds *[]api.BaseItemKind) []string {
	if kinds == nil {
		return nil
	}

	types := make([]string, 0, len(*kinds))
	for _, kind := range *kinds {
		types = append(types, string(kind))
	}

	return types
}

func emptyResult() api.BaseItemDtoQueryResult {
	return api.BaseItemDtoQueryResult{
		Items:            &[]api.BaseItemDto{},
		StartIndex:       dtos.Ptr(int32(0)),
		TotalRecordCount: dtos.Ptr(int32(0)),
	}
}
