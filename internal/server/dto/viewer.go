package dto

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type Access interface {
	Access(ctx context.Context, id uuid.UUID) (users.Access, error)
}

func ViewerFor(ctx context.Context, policies Access) (items.Viewer, error) {
	access, err := policies.Access(ctx, auth.UserID(ctx))
	if err != nil {
		return items.Viewer{}, err
	}

	return items.Viewer{All: access.All, Libraries: access.Libraries}, nil
}
