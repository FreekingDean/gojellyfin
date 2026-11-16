package dto

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
)

func CurrentUserDatum(ctx context.Context, store *items.Service, itemID uuid.UUID) (*items.Datum, error) {
	userID, err := SignedIn(ctx)
	if err != nil {
		return nil, err
	}

	return store.UserItemDatum(ctx, userID, itemID)
}

func SignedIn(ctx context.Context) (uuid.UUID, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return uuid.Nil, auth.ErrUnauthorized
	}

	return userID, nil
}
