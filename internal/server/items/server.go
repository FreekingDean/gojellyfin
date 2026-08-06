package items

import (
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

type Server struct {
	items     *items.Service
	libraries *libraries.Service
}

func New(service *items.Service, libraries *libraries.Service) *Server {
	return &Server{items: service, libraries: libraries}
}
