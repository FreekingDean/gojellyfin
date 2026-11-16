package musicgenres

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items    *items.Service
	policies dto.Access
}

func New(items *items.Service, policies dto.Access) *Server {
	return &Server{items: items, policies: policies}
}

var music = []api.BaseItemKind{api.BaseItemKindMusicArtist, api.BaseItemKindMusicAlbum, api.BaseItemKindAudio}

func (s *Server) GetMusicGenre(ctx context.Context, request api.GetMusicGenreRequestObject) (api.GetMusicGenreResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	genre, err := s.items.GenreNamed(ctx, items.MetadataQuery{Viewer: viewer, Kinds: dto.Kinds(&music)}, request.GenreName)
	if err != nil {
		return nil, err
	}

	return api.GetMusicGenre200JSONResponse(dto.NamedItem(genre, api.BaseItemKindMusicGenre, true)), nil
}
