package playlists

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) AddItemToPlaylist(ctx context.Context, request api.AddItemToPlaylistRequestObject) (api.AddItemToPlaylistResponseObject, error) {
	return api.AddItemToPlaylist204Response{}, nil
}
