package userlibrary

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
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
		int(apiutil.Deref(request.Params.StartIndex)),
		int(apiutil.Deref(request.Params.Limit)),
	)
	if err != nil {
		return nil, err
	}

	items, err := serveritems.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}

	return api.GetResumeItems200JSONResponse{
		Items:            &items,
		StartIndex:       apiutil.Ptr(int32(apiutil.Deref(request.Params.StartIndex))),
		TotalRecordCount: apiutil.Ptr(int32(total)),
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
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(0)),
	}
}
