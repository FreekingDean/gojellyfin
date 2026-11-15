package session

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
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
