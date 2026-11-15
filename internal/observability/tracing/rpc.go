package tracing

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type outgoing struct {
	md metadata.MD
}

func (o outgoing) Get(key string) string {
	values := o.md.Get(key)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (o outgoing) Set(key, value string) {
	o.md.Set(key, value)
}

func (o outgoing) Keys() []string {
	keys := make([]string, 0, len(o.md))
	for key := range o.md {
		keys = append(keys, key)
	}

	return keys
}

func UnaryClientInterceptor(
	ctx context.Context,
	method string,
	request, reply any,
	conn *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	options ...grpc.CallOption,
) error {
	if !within(ctx) {
		return invoker(ctx, method, request, reply, conn, options...)
	}

	name := strings.TrimPrefix(method, "/")
	ctx, span := child(ctx, name, trace.SpanKindClient,
		semconv.RPCSystemNameGRPC,
		semconv.RPCMethodKey.String(name),
	)
	defer span.End()

	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	carried.Inject(ctx, outgoing{md: md})
	ctx = metadata.NewOutgoingContext(ctx, md)

	err := invoker(ctx, method, request, reply, conn, options...)
	span.SetAttributes(semconv.RPCResponseStatusCodeKey.String(status.Code(err).String()))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "")
	}

	return err
}
