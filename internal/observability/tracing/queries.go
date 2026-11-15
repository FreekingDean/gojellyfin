package tracing

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

type queryKey struct{}

type Queries struct{}

func (Queries) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !within(ctx) {
		return ctx
	}

	operation := strings.ToUpper(strings.SplitN(strings.TrimSpace(data.SQL), " ", 2)[0])
	ctx, span := child(ctx, operation, trace.SpanKindClient,
		semconv.DBSystemNamePostgreSQL,
		semconv.DBOperationNameKey.String(operation),
		semconv.DBQueryTextKey.String(data.SQL),
	)

	return context.WithValue(ctx, queryKey{}, span)
}

func (Queries) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, ok := ctx.Value(queryKey{}).(trace.Span)
	if !ok {
		return
	}
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, "")
	}
	span.End()
}
