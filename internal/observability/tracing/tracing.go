package tracing

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/system"
)

const (
	serviceName     = "gojellyfin"
	shutdownTimeout = 5 * time.Second
)

const tracesPath = "/v1/traces"

type Tracing struct {
	provider   *sdktrace.TracerProvider
	propagator propagation.TextMapPropagator
}

func New(config env.Config) (*Tracing, error) {
	tracing := &Tracing{propagator: propagation.TraceContext{}}

	endpoint := config.Tracing.OTLPEndpoint
	if endpoint == "" {
		return tracing, nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT must be an http or https URL such as http://collector:4318, got %q", endpoint)
	}

	exporter, err := otlptracehttp.New(context.Background(), otlptracehttp.WithEndpointURL(tracesURL(parsed)))
	if err != nil {
		return nil, err
	}

	described, err := describe(config.Tracing)
	if err != nil {
		return nil, err
	}

	tracing.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(described),
	)

	return tracing, nil
}

func tracesURL(endpoint *url.URL) string {
	traces := *endpoint
	if !strings.HasSuffix(traces.Path, tracesPath) {
		traces.Path = strings.TrimSuffix(traces.Path, "/") + tracesPath
	}

	return traces.String()
}

func describe(config env.Tracing) (*resource.Resource, error) {
	name := config.ServiceName
	if name == "" {
		name = serviceName
	}

	attributes := []attribute.KeyValue{semconv.ServiceName(name), semconv.ServiceVersion(system.Build())}
	for pair := range strings.SplitSeq(config.ResourceAttributes, ",") {
		if strings.TrimSpace(pair) == "" {
			continue
		}
		key, value, split := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		decoded, err := url.PathUnescape(strings.TrimSpace(value))
		if !split || key == "" || err != nil {
			return nil, fmt.Errorf("OTEL_RESOURCE_ATTRIBUTES must be a comma separated list of key=value pairs such as k8s.pod.ip=10.0.0.1, got %q", config.ResourceAttributes)
		}
		attributes = append(attributes, attribute.String(key, decoded))
	}

	return resource.NewWithAttributes(semconv.SchemaURL, attributes...), nil
}

func (t *Tracing) Enabled() bool {
	return t.provider != nil
}

func (t *Tracing) StartRequest(ctx context.Context, header http.Header, operation, method string) (context.Context, *Span) {
	ctx = t.propagator.Extract(ctx, propagation.HeaderCarrier(header))
	ctx, span := t.Tracer().Start(ctx, operation,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(semconv.HTTPRequestMethodKey.String(method)),
	)

	return ctx, &Span{span: span}
}

type Span struct {
	span trace.Span
}

func (s *Span) End() {
	s.span.End()
}

func (s *Span) Fail(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, "")
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

func (t *Tracing) Stop(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	return t.provider.Shutdown(ctx)
}
