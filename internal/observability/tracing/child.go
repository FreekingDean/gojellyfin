package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var carried = propagation.TraceContext{}

func within(ctx context.Context) bool {
	return trace.SpanContextFromContext(ctx).IsValid()
}

func child(ctx context.Context, name string, kind trace.SpanKind, attributes ...attribute.KeyValue) (context.Context, trace.Span) {
	return trace.SpanFromContext(ctx).TracerProvider().Tracer(serviceName).Start(ctx, name,
		trace.WithSpanKind(kind),
		trace.WithAttributes(attributes...),
	)
}

func Start(ctx context.Context, name string) (context.Context, *Span) {
	if !within(ctx) {
		return ctx, &Span{span: trace.SpanFromContext(ctx)}
	}

	ctx, span := child(ctx, name, trace.SpanKindInternal)

	return ctx, &Span{span: span}
}

func Detach(ctx context.Context, name string) (context.Context, *Span) {
	if !within(ctx) {
		return ctx, &Span{span: trace.SpanFromContext(ctx)}
	}

	link := trace.LinkFromContext(ctx)
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer(serviceName).Start(ctx, name,
		trace.WithNewRoot(),
		trace.WithLinks(link),
		trace.WithSpanKind(trace.SpanKindProducer),
	)

	return ctx, &Span{span: span}
}
