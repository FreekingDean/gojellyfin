package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func (s *Server) RefreshItem(ctx context.Context, request api.RefreshItemRequestObject) (api.RefreshItemResponseObject, error) {
	if _, err := s.libraries.Library(ctx, request.ItemId); err != nil {
		if _, err := s.items.ItemByID(ctx, items.Everyone, request.ItemId); err != nil {
			return api.RefreshItem404JSONResponse{}, nil
		}
	}

	switch apiutil.Deref(request.Params.MetadataRefreshMode) {
	case api.MetadataRefreshModeDefault, api.MetadataRefreshModeValidationOnly:
		if err := s.tasks.Start(ctx, jobs.RefreshLibraries); err != nil {
			return nil, err
		}
	case api.MetadataRefreshModeFullRefresh:
		if err := s.tasks.Start(ctx, jobs.RefreshMetadata,
			jobs.With(jobs.ParamScope, request.ItemId),
			jobs.With(jobs.ParamForce, apiutil.Deref(request.Params.ReplaceAllMetadata)),
		); err != nil {
			return nil, err
		}
	}

	return api.RefreshItem204Response{}, nil
}
