package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrUnauthorized = errors.New("unauthorized")

type contextKey int

const (
	sessionKey contextKey = iota
	authorizationKey
)

// What the caller presented. Parsed at the transport edge, but owned here so
// handlers read it from this package rather than from the middleware.
type Authorization struct {
	Client   string
	Device   string
	DeviceID string
	Version  string
	Token    string
}

// Authenticate resolves a token and returns a context carrying the session.
// This package puts the identity on the context and takes it back off; nothing
// else should know the keys exist.
func (s *Service) Authenticate(ctx context.Context, token string) (context.Context, error) {
	if token == "" {
		return ctx, ErrUnauthorized
	}

	session, err := s.SessionByToken(ctx, token)
	if err != nil {
		return ctx, ErrUnauthorized
	}

	return context.WithValue(ctx, sessionKey, session), nil
}

func ContextWithAuthorization(ctx context.Context, authorization Authorization) context.Context {
	return context.WithValue(ctx, authorizationKey, authorization)
}

func AuthorizationFrom(ctx context.Context) Authorization {
	authorization, _ := ctx.Value(authorizationKey).(Authorization)

	return authorization
}

func SessionFrom(ctx context.Context) *Session {
	session, _ := ctx.Value(sessionKey).(*Session)

	return session
}

func UserID(ctx context.Context) uuid.UUID {
	if session := SessionFrom(ctx); session != nil {
		return session.Edges.User.ID
	}

	return uuid.Nil
}
