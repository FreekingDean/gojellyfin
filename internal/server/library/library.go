package library

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/tasks"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	items      *items.Service
	libraries  *libraries.Service
	users      *users.Service
	filesystem *filesystem.Service
	registry   *tasks.Registry
}

func New(
	items *items.Service,
	libraries *libraries.Service,
	users *users.Service,
	filesystem *filesystem.Service,
	registry *tasks.Registry,
) *Server {
	return &Server{items: items, libraries: libraries, users: users, filesystem: filesystem, registry: registry}
}

func (s *Server) RefreshLibrary(ctx context.Context, request api.RefreshLibraryRequestObject) (api.RefreshLibraryResponseObject, error) {
	if err := s.registry.Start(tasks.LibraryScanID); err != nil {
		return nil, err
	}

	return api.RefreshLibrary204Response{}, nil
}

func (s *Server) itemsByID(ctx context.Context, ids []uuid.UUID) ([]*items.Item, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	records, _, err := s.items.QueryItems(ctx, items.ItemQuery{IDs: ids})

	return records, err
}
