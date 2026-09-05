package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

func (s *Server) GetAncestors(ctx context.Context, request api.GetAncestorsRequestObject) (api.GetAncestorsResponseObject, error) {
	ancestry, err := s.items.Ancestors(ctx, request.ItemId)
	if err != nil {
		return nil, err
	}
	if ancestry == nil {
		return api.GetAncestors404JSONResponse{}, nil
	}

	converted, err := dto.ItemDtos(ctx, s.items, ancestry.Parents)
	if err != nil {
		return nil, err
	}

	if len(ancestry.LibraryIDs) > 0 {
		library, err := s.libraries.Library(ctx, ancestry.LibraryIDs[0])
		if err != nil {
			return nil, err
		}
		converted = append(converted, dto.LibraryView(library))
	}

	return api.GetAncestors200JSONResponse(append(converted, dto.RootView())), nil
}
