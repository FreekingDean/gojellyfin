package dto

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
)

func CurrentUserDatum(ctx context.Context, store *items.Service, itemID uuid.UUID) (*items.Datum, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return nil, auth.ErrUnauthorized
	}

	return store.UserItemDatum(ctx, userID, itemID)
}
