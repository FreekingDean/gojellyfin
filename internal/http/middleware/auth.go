package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

var ErrUnauthorized = errors.New("unauthorized")

type contextKey int

const (
	authorizationKey contextKey = iota
	sessionKey
)

type Authorization struct {
	Client   string
	Device   string
	DeviceID string
	Version  string
	Token    string
}

// Identity of the caller, resolved once per request. Deliberately not the user
// record itself, so this package stays independent of the domain packages.
type Session struct {
	ID     uuid.UUID
	UserID uuid.UUID
}

type Sessions interface {
	SessionByToken(ctx context.Context, token string) (Session, error)
}

type Auth struct {
	sessions Sessions
}

func NewAuth(sessions Sessions) *Auth {
	return &Auth{sessions: sessions}
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

		session, err := a.sessions.SessionByToken(ctx, authorization.Token)
		if err != nil {
			return nil, ErrUnauthorized
		}

		return f(context.WithValue(ctx, sessionKey, session), w, r, request)
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

func SessionFrom(ctx context.Context) Session {
	session, _ := ctx.Value(sessionKey).(Session)

	return session
}

func UserID(ctx context.Context) uuid.UUID {
	return SessionFrom(ctx).UserID
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
