package tracing

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

const (
	serviceName     = "gojellyfin"
	shutdownTimeout = 5 * time.Second
)

type Tracing struct {
	provider   *sdktrace.TracerProvider
	propagator propagation.TextMapPropagator
}

// No endpoint means tracing is off, the way an empty TEMPORAL_HOSTPORT means
// background work is off: a developer running the server alone gets a server,
// not a dial error on every start.
func New(config env.Config) (*Tracing, error) {
	tracing := &Tracing{propagator: propagation.TraceContext{}}

	endpoint := config.Tracing.OTLPEndpoint
	if endpoint == "" {
		return tracing, nil
	}

	// The exporter answers a URL it cannot parse by logging and carrying on
	// against its own localhost default, so a typo would otherwise ship every
	// trace nowhere with nothing to point at.
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT must be an http or https URL such as http://collector:4318, got %q", endpoint)
	}

	exporter, err := otlptracehttp.New(context.Background(), otlptracehttp.WithEndpointURL(endpoint))
	if err != nil {
		return nil, err
	}

	tracing.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		)),
	)

	return tracing, nil
}

func (t *Tracing) Enabled() bool {
	return t.provider != nil
}

func (t *Tracing) Tracer() trace.Tracer {
	if t.provider == nil {
		return noop.NewTracerProvider().Tracer(serviceName)
	}

	return t.provider.Tracer(serviceName)
}

func (t *Tracing) Propagator() propagation.TextMapPropagator {
	return t.propagator
}

func (t *Tracing) Start() error {
	return nil
}

// A collector that has gone away must not hold the process open, so the flush
// is bounded rather than waiting on the exporter's own retries.
func (t *Tracing) Stop() error {
	if t.provider == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return t.provider.Shutdown(ctx)
}
