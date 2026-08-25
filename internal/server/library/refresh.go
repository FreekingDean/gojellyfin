package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/metadata"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

// The id names a library or an item, because that is what the client sends;
// which it is falls out of the scope predicate rather than being worked out
// here. Everything the provider returns is written, so there is no difference
// between asking again and replacing: replaceAllMetadata decides whether items
// that already have metadata are looked up again at all.
func (s *Server) RefreshItem(ctx context.Context, request api.RefreshItemRequestObject) (api.RefreshItemResponseObject, error) {
	if _, err := s.libraries.Library(ctx, request.ItemId); err != nil {
		if _, err := s.items.ItemByID(ctx, request.ItemId); err != nil {
			return api.RefreshItem404JSONResponse{}, nil
		}
	}

	switch apiutil.Deref(request.Params.MetadataRefreshMode) {
	case api.MetadataRefreshModeDefault, api.MetadataRefreshModeValidationOnly:
		if err := s.tasks.Start(ctx, scanner.RefreshLibraryJobID, jobs.Options{}); err != nil {
			return nil, err
		}
	case api.MetadataRefreshModeFullRefresh:
		if err := s.tasks.Start(ctx, metadata.RefreshMetadataJobID, jobs.Options{
			Scope: request.ItemId,
			Force: apiutil.Deref(request.Params.ReplaceAllMetadata),
		}); err != nil {
			return nil, err
		}
	}

	return api.RefreshItem204Response{}, nil
}
