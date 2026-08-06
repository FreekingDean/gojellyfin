package items

import (
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

type Server struct {
	store     *items.Store
	libraries *libraries.Store
}

func New(store *items.Store, libraries *libraries.Store) *Server {
	return &Server{store: store, libraries: libraries}
}
