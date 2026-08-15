package tracing

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

// A developer running the server alone gets a server: no collector configured
// means no provider to shut down and a tracer that records nothing, so nothing
// is held and nothing is dialed.
func TestNoEndpointRecordsNothing(t *testing.T) {
	tracing, err := New(env.Config{})
	if err != nil {
		t.Fatalf("an unconfigured collector failed to build: %v", err)
	}

	_, span := tracing.Tracer().Start(context.Background(), "check")
	defer span.End()

	if span.IsRecording() {
		t.Error("a span is recording with no collector configured")
	}
	if err := tracing.Stop(); err != nil {
		t.Errorf("stopping an unconfigured tracer failed: %v", err)
	}
}

func TestAnEndpointRecords(t *testing.T) {
	tracing, err := New(env.Config{
		Tracing: env.Tracing{OTLPEndpoint: "http://collector.example:4318"},
	})
	if err != nil {
		t.Fatalf("failed to build: %v", err)
	}
	defer tracing.Stop()

	_, span := tracing.Tracer().Start(context.Background(), "check")
	defer span.End()

	if !span.IsRecording() {
		t.Error("a collector is configured and nothing is recorded")
	}
}
