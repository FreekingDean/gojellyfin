package tracing

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func configured(endpoint string) env.Config {
	return env.Config{Tracing: env.Tracing{OTLPEndpoint: endpoint}}
}

// A developer running the server alone gets a server: no collector configured
// means no provider to shut down and a tracer that records nothing, so nothing
// is held and nothing is dialed.
func TestNoEndpointRecordsNothing(t *testing.T) {
	tracing, err := New(env.Config{})
	if err != nil {
		t.Fatalf("an unconfigured collector failed to build: %v", err)
	}

	if tracing.Enabled() {
		t.Error("tracing reports itself enabled with no collector configured")
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
	tracing, err := New(configured("http://collector.example:4318"))
	if err != nil {
		t.Fatalf("failed to build: %v", err)
	}
	t.Cleanup(func() {
		if err := tracing.Stop(); err != nil {
			t.Errorf("stopping failed: %v", err)
		}
	})

	if !tracing.Enabled() {
		t.Error("a collector is configured and tracing reports itself off")
	}

	_, span := tracing.Tracer().Start(context.Background(), "check")
	defer span.End()

	if !span.IsRecording() {
		t.Error("a collector is configured and nothing is recorded")
	}
}

// The exporter answers an unparseable URL by logging and carrying on against
// localhost, so a typo would otherwise be silent.
func TestMalformedEndpointIsRefused(t *testing.T) {
	for _, endpoint := range []string{"collector:4318", "grpc://collector:4318", "http://", "not a url"} {
		t.Run(endpoint, func(t *testing.T) {
			if _, err := New(configured(endpoint)); err == nil {
				t.Errorf("%q was accepted", endpoint)
			}
		})
	}
}
