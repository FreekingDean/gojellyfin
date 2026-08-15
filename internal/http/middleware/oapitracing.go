package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

// Everything under these two roots is a progressive response that runs for the
// length of the media, so a span covering one is open for hours. They are the
// same roots deploy/httproutes.yaml sends to the transcode pods, which is what
// keeps the rule to one line instead of a list of operations.
var untracedRoots = []string{"Videos", "Audio"}

type OapiTracing struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

func NewOapiTracing(t *tracing.Tracing) *OapiTracing {
	return &OapiTracing{tracer: t.Tracer(), propagator: t.Propagator()}
}

func (t *OapiTracing) Middleware(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		if streams(r.URL.Path) {
			return f(ctx, w, r, request)
		}

		ctx = t.propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))
		ctx, span := t.tracer.Start(ctx, operationID,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(semconv.HTTPRequestMethodKey.String(r.Method)),
		)
		defer span.End()

		resp, err := f(ctx, w, r, request)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "")
		}

		return resp, err
	}
}

// The mux matches case-insensitively, so /videos reaches the same handler as
// /Videos and has to be excluded the same way.
func streams(path string) bool {
	root, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")

	return slices.ContainsFunc(untracedRoots, func(untraced string) bool {
		return strings.EqualFold(untraced, root)
	})
}
