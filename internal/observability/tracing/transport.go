package tracing

import (
	"net/http"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

type transport struct {
	base http.RoundTripper
}

func Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return transport{base: base}
}

func (t transport) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx := request.Context()
	if !within(ctx) {
		return t.base.RoundTrip(request)
	}

	ctx, span := child(ctx, request.Method+" "+request.URL.Host, trace.SpanKindClient,
		semconv.HTTPRequestMethodKey.String(request.Method),
		semconv.ServerAddressKey.String(request.URL.Hostname()),
		semconv.URLPathKey.String(request.URL.Path),
	)
	defer span.End()

	outbound := request.Clone(ctx)
	carried.Inject(ctx, propagation.HeaderCarrier(outbound.Header))

	response, err := t.base.RoundTrip(outbound)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "")

		return nil, err
	}

	span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(response.StatusCode))
	if response.StatusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, "")
	}

	return response, nil
}
