package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

// Transport only: parse what the client sent and hand it to auth, which owns
// the identity on the context.
type Auth struct {
	auth *auth.Service
}

func NewAuth(auth *auth.Service) *Auth {
	return &Auth{auth: auth}
}

func (a *Auth) Middleware(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		authorization := parseAuthorization(r)
		ctx = auth.ContextWithAuthorization(ctx, authorization)

		if api.PublicOperations[operationID] {
			return f(ctx, w, r, request)
		}

		ctx, err := a.auth.Authenticate(ctx, authorization.Token)
		if err != nil {
			return nil, err
		}

		return f(ctx, w, r, request)
	}
}

// Media URLs and websocket handshakes cannot carry an Authorization header, so
// clients fall back to an api_key query parameter.
func TokenFrom(r *http.Request) string {
	if token := parseAuthorization(r).Token; token != "" {
		return token
	}

	return r.URL.Query().Get("api_key")
}

func parseAuthorization(r *http.Request) auth.Authorization {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "MediaBrowser") {
		header = r.Header.Get("X-Emby-Authorization")
	}

	authorization := auth.Authorization{}
	for _, pair := range strings.Split(strings.TrimPrefix(header, "MediaBrowser"), ",") {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}

		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "client":
			authorization.Client = value
		case "device":
			authorization.Device = value
		case "deviceid":
			authorization.DeviceID = value
		case "version":
			authorization.Version = value
		case "token":
			authorization.Token = value
		}
	}

	if authorization.Token == "" {
		authorization.Token = r.Header.Get("X-Emby-Token")
	}
	if authorization.Token == "" {
		authorization.Token = r.Header.Get("X-MediaBrowser-Token")
	}

	return authorization
}
