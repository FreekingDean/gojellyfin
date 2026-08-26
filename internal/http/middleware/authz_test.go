package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

type stubPolicies struct {
	allow  bool
	scopes []string
}

func (s *stubPolicies) Satisfies(_ context.Context, _ uuid.UUID, scopes []string) (bool, error) {
	s.scopes = scopes

	return s.allow, nil
}

func run(t *testing.T, policies Policies, operationID string, ctx context.Context) error {
	t.Helper()

	reached := false
	handler := Authorize(policies)(func(context.Context, http.ResponseWriter, *http.Request, any) (any, error) {
		reached = true

		return nil, nil
	}, operationID)

	_, err := handler(ctx, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), nil)
	if err == nil && !reached {
		t.Fatal("the handler was neither reached nor refused")
	}
	if err != nil && reached {
		t.Fatal("the handler ran despite being refused")
	}

	return err
}

func authenticated() context.Context {
	session := &sessions.Session{}
	session.Edges.User = &store.User{ID: uuid.New()}

	return auth.ContextWithSession(context.Background(), session)
}

func TestAuthorize(t *testing.T) {
	t.Run("a public operation skips authorization", func(t *testing.T) {
		var operationID string
		for name := range api.PublicOperations {
			operationID = name
			break
		}

		if err := run(t, &stubPolicies{}, operationID, context.Background()); err != nil {
			t.Errorf("a public operation was refused: %v", err)
		}
	})

	t.Run("an unknown operation is refused", func(t *testing.T) {
		err := run(t, &stubPolicies{allow: true}, "SomeOperationTheSpecNoLongerDeclares", context.Background())

		if !errors.Is(err, auth.ErrForbidden) {
			t.Errorf("got %v, want ErrForbidden", err)
		}
	})

	t.Run("an unauthenticated caller is refused", func(t *testing.T) {
		err := run(t, &stubPolicies{allow: true}, "UpdateUserPolicy", context.Background())

		if !errors.Is(err, auth.ErrUnauthorized) {
			t.Errorf("got %v, want ErrUnauthorized", err)
		}
	})

	t.Run("the operation scopes are handed to the policies", func(t *testing.T) {
		policies := &stubPolicies{allow: true}
		ctx := authenticated()

		if err := run(t, policies, "UpdateUserPolicy", ctx); err != nil {
			t.Fatalf("an allowed caller was refused: %v", err)
		}

		if len(policies.scopes) == 0 {
			t.Fatal("no scopes were checked")
		}
		if policies.scopes[0] != "RequiresElevation" {
			t.Errorf("scopes = %v, want RequiresElevation", policies.scopes)
		}
	})

	t.Run("a refused caller gets forbidden", func(t *testing.T) {
		ctx := authenticated()

		err := run(t, &stubPolicies{allow: false}, "UpdateUserPolicy", ctx)
		if !errors.Is(err, auth.ErrForbidden) {
			t.Errorf("got %v, want ErrForbidden", err)
		}
	})
}
