package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Policies interface {
	Satisfies(ctx context.Context, id uuid.UUID, scopes []string) (bool, error)
}

func Authorize(policies Policies) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			if api.PublicOperations[operationID] {
				return next(ctx, w, r, request)
			}

			scopes, ok := api.OperationPolicies[operationID]
			if !ok {
				return nil, auth.ErrForbidden
			}

			id := auth.UserID(ctx)
			if id == uuid.Nil {
				return nil, auth.ErrUnauthorized
			}

			allowed, err := policies.Satisfies(ctx, id, scopes)
			if err != nil {
				return nil, err
			}
			if !allowed {
				return nil, auth.ErrForbidden
			}

			return next(ctx, w, r, request)
		}
	}
}
