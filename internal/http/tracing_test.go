package http

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

// With no collector configured the span middleware is left out of the stack
// entirely rather than wrapping every request in a tracer that discards it.
func TestTracingMiddlewareIsOnlyInstalledWhenEnabled(t *testing.T) {
	off, err := tracing.New(env.Config{})
	if err != nil {
		t.Fatal(err)
	}
	on, err := tracing.New(env.Config{
		Tracing: env.Tracing{OTLPEndpoint: "http://collector.example:4318"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := on.Stop(context.Background()); err != nil {
			t.Errorf("stopping failed: %v", err)
		}
	})

	var auth *middleware.Auth

	unconfigured := newAPIMiddleware(off, middleware.NewOapiTracing(off), auth, nil)
	configured := newAPIMiddleware(on, middleware.NewOapiTracing(on), auth, nil)

	if len(configured) != len(unconfigured)+1 {
		t.Errorf("stack is %d with a collector and %d without, want one more", len(configured), len(unconfigured))
	}
}
