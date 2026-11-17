package tracing

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

type Starter = func(ctx context.Context, tracer trace.Tracer, name string, opts ...trace.SpanStartOption) trace.Span

func Starts(passed, rooted []string) Starter {
	return func(ctx context.Context, tracer trace.Tracer, name string, opts ...trace.SpanStartOption) trace.Span {
		if prefixed(name, passed) {
			return trace.SpanFromContext(ctx)
		}
		if prefixed(name, rooted) {
			opts = append(opts, trace.WithNewRoot(), trace.WithSpanKind(trace.SpanKindConsumer))
			if within(ctx) {
				opts = append(opts, trace.WithLinks(trace.LinkFromContext(ctx)))
			}
		}
		_, span := tracer.Start(ctx, name, opts...)

		return span
	}
}

func prefixed(name string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

func Current(ctx context.Context) *Span {
	return &Span{span: trace.SpanFromContext(ctx)}
}
