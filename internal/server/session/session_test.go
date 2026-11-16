package session

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

func TestServer_GetSessions_ControllableByNobody(t *testing.T) {
	controller := uuid.New()

	response, err := (&Server{}).GetSessions(context.Background(), api.GetSessionsRequestObject{
		Params: api.GetSessionsParams{ControllableByUserId: &controller},
	})
	if err != nil {
		t.Fatalf("GetSessions: %v", err)
	}

	sessions, ok := response.(api.GetSessions200JSONResponse)
	if !ok {
		t.Fatalf("GetSessions answered %T, want a 200", response)
	}
	if len(sessions) != 0 {
		t.Errorf("GetSessions answered %d controllable sessions, want none while no command reaches a session", len(sessions))
	}
}

type nobodyAdministers struct{}

func (nobodyAdministers) IsAdministrator(context.Context, uuid.UUID) (bool, error) { return false, nil }

func TestServer_owned(t *testing.T) {
	own := &sessions.Session{ID: uuid.New()}
	ctx := auth.ContextWithSession(context.Background(), own)
	server := &Server{administrators: nobodyAdministers{}}

	if _, allowed, err := server.owned(ctx, own.ID.String()); err != nil || !allowed {
		t.Errorf("owned(own session) = %v, %v, want allowed", allowed, err)
	}
	if _, allowed, err := server.owned(ctx, "not-a-session"); err != nil || allowed {
		t.Errorf("owned(garbage) = %v, %v, want refused", allowed, err)
	}
}
