package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

func (s *Server) GetItemCollections(ctx context.Context, request api.GetItemCollectionsRequestObject) (api.GetItemCollectionsResponseObject, error) {
	return api.GetItemCollections200JSONResponse(dto.EmptyItems()), nil
}
