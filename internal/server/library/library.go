package library

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Server struct {
	items      *items.Service
	libraries  *libraries.Service
	users      *users.Service
	filesystem *filesystem.Service
	tasks      *jobs.Service
}

func New(
	items *items.Service,
	libraries *libraries.Service,
	users *users.Service,
	filesystem *filesystem.Service,
	service *jobs.Service,
) *Server {
	return &Server{items: items, libraries: libraries, users: users, filesystem: filesystem, tasks: service}
}

func (s *Server) RefreshLibrary(ctx context.Context, request api.RefreshLibraryRequestObject) (api.RefreshLibraryResponseObject, error) {
	if err := s.tasks.Start(ctx, scanner.RefreshLibraryJobID); err != nil {
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
