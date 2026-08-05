package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

var ErrUnauthorized = errors.New("unauthorized")

type contextKey int

const (
	authorizationKey contextKey = iota
	userKey
	sessionKey
)

type Authorization struct {
	Client   string
	Device   string
	DeviceID string
	Version  string
	Token    string
}

type Auth struct {
	store store.Store
}

func NewAuth(store store.Store) *Auth {
	return &Auth{store: store}
}

func (a *Auth) Middleware(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		authorization := parseAuthorization(r)
		ctx = context.WithValue(ctx, authorizationKey, authorization)

		if api.PublicOperations[operationID] {
			return f(ctx, w, r, request)
		}

		if authorization.Token == "" {
			return nil, ErrUnauthorized
		}

		session, err := a.store.GetSessionByToken(ctx, authorization.Token)
		if err != nil {
			return nil, ErrUnauthorized
		}

		user, err := a.store.GetUser(ctx, session.UserID)
		if err != nil {
			return nil, ErrUnauthorized
		}

		ctx = context.WithValue(ctx, sessionKey, session)
		ctx = context.WithValue(ctx, userKey, user)

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

func AuthorizationFrom(ctx context.Context) Authorization {
	authorization, _ := ctx.Value(authorizationKey).(Authorization)

	return authorization
}

func UserFrom(ctx context.Context) *store.User {
	user, _ := ctx.Value(userKey).(*store.User)

	return user
}

func SessionFrom(ctx context.Context) *store.Session {
	session, _ := ctx.Value(sessionKey).(*store.Session)

	return session
}

func parseAuthorization(r *http.Request) Authorization {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "MediaBrowser") {
		header = r.Header.Get("X-Emby-Authorization")
	}

	authorization := Authorization{}
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
