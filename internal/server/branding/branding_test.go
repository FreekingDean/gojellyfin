package branding

import (
	"context"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

type signOn bool

func (s signOn) Enabled() bool { return bool(s) }

func TestServer_GetBrandingOptions(t *testing.T) {
	base, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}
	connection, err := store.NewStore(base)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}
	t.Cleanup(func() { _ = connection.Stop() })

	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{name: "offers the button when sign-on is configured", enabled: true, want: true},
		{name: "offers nothing when it is not", enabled: false, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{config: config.New(connection.Client()), signOn: signOn(test.enabled)}

			response, err := server.GetBrandingOptions(context.Background(), api.GetBrandingOptionsRequestObject{})
			if err != nil {
				t.Fatalf("GetBrandingOptions returned %v", err)
			}
			disclaimer := apiutil.Deref(response.(api.GetBrandingOptions200JSONResponse).LoginDisclaimer)

			if got := strings.Contains(disclaimer, `action="/sso/oidc/start"`); got != test.want {
				t.Errorf("button offered = %v, want %v in %q", got, test.want, disclaimer)
			}
		})
	}
}
