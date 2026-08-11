package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/tasks"
)

const ScanTaskID = "RefreshLibrary"

type Server struct {
	items     *items.Service
	libraries *libraries.Service
	registry  *tasks.Registry
}

func New(items *items.Service, libraries *libraries.Service, registry *tasks.Registry) *Server {
	return &Server{items: items, libraries: libraries, registry: registry}
}

func (s *Server) RefreshLibrary(ctx context.Context, request api.RefreshLibraryRequestObject) (api.RefreshLibraryResponseObject, error) {
	if err := s.registry.Start(ScanTaskID); err != nil {
		return nil, err
	}

	return api.RefreshLibrary204Response{}, nil
}
