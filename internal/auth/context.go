package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

type contextKey int

const (
	sessionKey contextKey = iota
	authorizationKey
)

type Authorization struct {
	Client     string
	Device     string
	DeviceID   string
	Version    string
	Token      string
	RemoteAddr string
}

func (a Authorization) DeviceInfo() sessions.DeviceInfo {
	return sessions.DeviceInfo{
		ID:         a.DeviceID,
		Name:       a.Device,
		AppName:    a.Client,
		AppVersion: a.Version,
	}
}

type Service struct {
	sessions *sessions.Service
}

func New(sessions *sessions.Service) *Service {
	return &Service{sessions: sessions}
}

func (s *Service) Authenticate(ctx context.Context, token string) (context.Context, error) {
	if token == "" {
		return ctx, ErrUnauthorized
	}

	session, err := s.sessions.ByToken(ctx, token)
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

func ContextWithSession(ctx context.Context, session *sessions.Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

func SessionFrom(ctx context.Context) *sessions.Session {
	session, _ := ctx.Value(sessionKey).(*sessions.Session)

	return session
}

func UserID(ctx context.Context) uuid.UUID {
	if session := SessionFrom(ctx); session != nil && session.Edges.User != nil {
		return session.Edges.User.ID
	}

	return uuid.Nil
}
