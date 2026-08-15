package musicgenres

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetMusicGenre(ctx context.Context, request api.GetMusicGenreRequestObject) (api.GetMusicGenreResponseObject, error) {
	genre, err := s.items.GenreByName(ctx, request.GenreName)
	if err != nil {
		return nil, err
	}

	return api.GetMusicGenre200JSONResponse(dto.GenreDto(genre, api.BaseItemKindMusicGenre)), nil
}
