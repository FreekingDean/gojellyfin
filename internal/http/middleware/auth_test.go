package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type setup bool

func (s setup) Completed(context.Context) (bool, error) {
	return bool(s), nil
}

func TestFirstTimeSetupOperationsCloseWithTheWizard(t *testing.T) {
	tests := map[string]struct {
		operation string
		completed bool
		handled   bool
	}{
		"startup before the wizard runs": {operation: "GetStartupConfiguration", completed: false, handled: true},
		"startup after the wizard runs":  {operation: "GetStartupConfiguration", completed: true, handled: false},
		"library setup before the wizard runs": {
			operation: "GetVirtualFolders", completed: false, handled: true,
		},
		"library setup after the wizard runs": {
			operation: "GetVirtualFolders", completed: true, handled: false,
		},
		"public stays open":      {operation: "GetPublicUsers", completed: true, handled: true},
		"everything else shut":   {operation: "GetItems", completed: false, handled: false},
		"unknown operation shut": {operation: "NotAnOperation", completed: false, handled: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			handled := false
			handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
				handled = true

				return nil, nil
			}

			middleware := NewAuth(auth.New(nil), setup(test.completed)).Middleware(handler, test.operation)
			_, err := middleware(context.Background(), httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil), nil)

			if handled != test.handled {
				t.Errorf("handler reached = %v, want %v", handled, test.handled)
			}
			if !test.handled && !errors.Is(err, auth.ErrUnauthorized) {
				t.Errorf("err = %v, want %v", err, auth.ErrUnauthorized)
			}
		})
	}
}

func TestFirstTimeSetupOperationsCoverTheWizard(t *testing.T) {
	for _, operation := range []string{
		"CompleteWizard",
		"GetFirstUser",
		"GetFirstUser2",
		"GetStartupConfiguration",
		"SetRemoteAccess",
		"UpdateInitialConfiguration",
		"UpdateStartupUser",
	} {
		if !api.FirstTimeSetupOperations[operation] {
			t.Errorf("%s is not a first time setup operation", operation)
		}
		if api.PublicOperations[operation] {
			t.Errorf("%s is public, so the wizard would never close", operation)
		}
	}
}

func TestTokenFromAcceptsEitherQuerySpelling(t *testing.T) {
	tests := map[string]string{
		"/socket?ApiKey=abc":            "abc",
		"/socket?api_key=abc":           "abc",
		"/socket?apikey=abc":            "abc",
		"/socket?deviceId=x&ApiKey=abc": "abc",
		"/socket":                       "",
	}

	for target, want := range tests {
		if got := TokenFrom(httptest.NewRequest("GET", target, nil)); got != want {
			t.Errorf("TokenFrom(%q) = %q, want %q", target, got, want)
		}
	}
}

func TestTokenFromPrefersTheHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/socket?ApiKey=fromquery", nil)
	r.Header.Set("Authorization", `MediaBrowser Token="fromheader", Client="test"`)

	if got := TokenFrom(r); got != "fromheader" {
		t.Errorf("TokenFrom = %q, want fromheader", got)
	}
}

func TestTokenFromDecodesPercentEncoding(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", `MediaBrowser Client="Jellyfin%20Web", Token="a%2Bb"`)

	if got := TokenFrom(r); got != "a+b" {
		t.Errorf("TokenFrom = %q, want %q", got, "a+b")
	}
}
