package artists

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetArtists(ctx context.Context, request api.GetArtistsRequestObject) (api.GetArtistsResponseObject, error) {
	return api.GetArtists200JSONResponse(dto.EmptyItems()), nil
}

func (s *Server) GetAlbumArtists(ctx context.Context, request api.GetAlbumArtistsRequestObject) (api.GetAlbumArtistsResponseObject, error) {
	return api.GetAlbumArtists200JSONResponse(dto.EmptyItems()), nil
}
