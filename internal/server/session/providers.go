package session

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

// Only the built-in provider exists; these names are what the dashboard shows
// against a user, and must match what DefaultPolicy reports.
func (s *Server) GetAuthProviders(ctx context.Context, request api.GetAuthProvidersRequestObject) (api.GetAuthProvidersResponseObject, error) {
	return api.GetAuthProviders200JSONResponse([]api.NameIdPair{{
		Name: apiutil.Ptr("Default"),
		Id:   apiutil.Ptr("Jellyfin.Server.Implementations.Users.DefaultAuthenticationProvider"),
	}}), nil
}

func (s *Server) GetPasswordResetProviders(ctx context.Context, request api.GetPasswordResetProvidersRequestObject) (api.GetPasswordResetProvidersResponseObject, error) {
	return api.GetPasswordResetProviders200JSONResponse([]api.NameIdPair{{
		Name: apiutil.Ptr("Default"),
		Id:   apiutil.Ptr("Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider"),
	}}), nil
}
