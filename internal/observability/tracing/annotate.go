package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Attribute struct {
	value attribute.KeyValue
}

func String(key, value string) Attribute {
	return Attribute{value: attribute.String(key, value)}
}

func Int(key string, value int) Attribute {
	return Attribute{value: attribute.Int(key, value)}
}

func Bool(key string, value bool) Attribute {
	return Attribute{value: attribute.Bool(key, value)}
}

func (s *Span) Set(attributes ...Attribute) {
	for _, a := range attributes {
		s.span.SetAttributes(a.value)
	}
}

func TraceID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ""
	}

	return spanContext.TraceID().String()
}

func (s *Span) Failed() {
	s.span.SetStatus(codes.Error, "")
}
