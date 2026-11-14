package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func inside(t *testing.T) (context.Context, *Span, *Recorder) {
	t.Helper()

	tracing, recorder := Recorded()
	ctx, span := tracing.StartRequest(context.Background(), http.Header{}, "GetItems", http.MethodGet)

	return ctx, span, recorder
}

func ended(recorder *Recorder, name string) (tracetest.SpanStub, bool) {
	for _, span := range tracetest.SpanStubsFromReadOnlySpans(recorder.recorder.Ended()) {
		if span.Name == name {
			return span, true
		}
	}

	return tracetest.SpanStub{}, false
}

func TestTransport(t *testing.T) {
	var traceparent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceparent = r.Header.Get("traceparent")
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: Transport(nil)}

	t.Run("outside a span it sends nothing and records nothing", func(t *testing.T) {
		request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/movie?api_key=secret", nil)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()

		if traceparent != "" {
			t.Errorf("traceparent = %q outside any span", traceparent)
		}
	})

	t.Run("inside a span it is a child that carries the trace and no query", func(t *testing.T) {
		ctx, parent, recorder := inside(t)
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/movie?api_key=secret", nil)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		parent.End()

		span, ok := ended(recorder, "GET "+request.URL.Host)
		if !ok {
			t.Fatalf("no client span among %v", recorder.Names())
		}
		if span.Parent.SpanID() != trace.SpanContextFromContext(ctx).SpanID() {
			t.Error("the client span is not a child of the request")
		}
		if traceparent == "" {
			t.Error("the trace was not propagated")
		}
		for _, attribute := range span.Attributes {
			if value := attribute.Value.Emit(); value == "secret" || value == server.URL+"/movie?api_key=secret" {
				t.Errorf("%s carries the query string", attribute.Key)
			}
		}
		if span.Status.Code.String() != "Error" {
			t.Errorf("a 404 is %s, want Error", span.Status.Code)
		}
	})
}

func TestUnaryClientInterceptor(t *testing.T) {
	var sent metadata.MD
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		sent, _ = metadata.FromOutgoingContext(ctx)

		return errors.New("refused")
	}
	method := "/temporal.api.workflowservice.v1.WorkflowService/StartWorkflowExecution"

	t.Run("outside a span the call goes untraced", func(t *testing.T) {
		_ = UnaryClientInterceptor(context.Background(), method, nil, nil, nil, invoker)
		if len(sent.Get("traceparent")) != 0 {
			t.Error("a call outside any span carried a traceparent")
		}
	})

	t.Run("inside a span the call is a child and carries the trace", func(t *testing.T) {
		ctx, parent, recorder := inside(t)
		if err := UnaryClientInterceptor(ctx, method, nil, nil, nil, invoker); err == nil {
			t.Fatal("the invoker's error was swallowed")
		}
		parent.End()

		span, ok := ended(recorder, method[1:])
		if !ok {
			t.Fatalf("no rpc span among %v", recorder.Names())
		}
		if span.Parent.SpanID() != trace.SpanContextFromContext(ctx).SpanID() {
			t.Error("the rpc span is not a child of the request")
		}
		if len(sent.Get("traceparent")) != 1 {
			t.Errorf("traceparent = %v, want one", sent.Get("traceparent"))
		}
	})
}

func TestQueries(t *testing.T) {
	t.Run("outside a span nothing is started and ending is a no-op", func(t *testing.T) {
		ctx := Queries{}.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
		Queries{}.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	})

	t.Run("inside a span a query is named for its verb and keeps no arguments", func(t *testing.T) {
		ctx, parent, recorder := inside(t)
		queried := Queries{}.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: " select * from item where key = $1", Args: []any{"secret"}})
		Queries{}.TraceQueryEnd(queried, nil, pgx.TraceQueryEndData{Err: errors.New("boom")})

		if _, ok := ended(recorder, "GetItems"); ok {
			t.Fatal("ending the query ended the request")
		}
		parent.End()

		span, ok := ended(recorder, "SELECT")
		if !ok {
			t.Fatalf("no query span among %v", recorder.Names())
		}
		for _, attribute := range span.Attributes {
			if attribute.Value.Emit() == "secret" {
				t.Errorf("%s carries an argument", attribute.Key)
			}
		}
	})
}

func TestStartAndDetach(t *testing.T) {
	t.Run("outside a span neither starts one", func(t *testing.T) {
		ctx, span := Start(context.Background(), "tmdb Movie")
		span.End()
		ctx, span = Detach(ctx, "Enqueue RefreshItem")
		span.End()
		if trace.SpanContextFromContext(ctx).IsValid() {
			t.Error("a span was started with no trace to belong to")
		}
	})

	t.Run("Start is a child and Detach a new trace linked to the old", func(t *testing.T) {
		ctx, parent, recorder := inside(t)
		_, started := Start(ctx, "tmdb Movie")
		started.End()
		_, detached := Detach(ctx, "Enqueue RefreshItem")
		detached.End()
		parent.End()

		internal, _ := ended(recorder, "tmdb Movie")
		if internal.SpanContext.TraceID() != trace.SpanContextFromContext(ctx).TraceID() {
			t.Error("Start left the trace")
		}

		producer, _ := ended(recorder, "Enqueue RefreshItem")
		if producer.SpanContext.TraceID() == trace.SpanContextFromContext(ctx).TraceID() {
			t.Error("Detach stayed in the enqueuing trace")
		}
		if len(producer.Links) != 1 || producer.Links[0].SpanContext.SpanID() != trace.SpanContextFromContext(ctx).SpanID() {
			t.Errorf("links = %v, want the enqueuing span", producer.Links)
		}
	})
}

func TestDescribe(t *testing.T) {
	described, err := describe(env.Tracing{ServiceName: "gojellyfin-worker", ResourceAttributes: "k8s.pod.ip=10.0.0.1, deployment.environment.name=home%20lab"})
	if err != nil {
		t.Fatal(err)
	}

	values := map[string]string{}
	for _, attribute := range described.Attributes() {
		values[string(attribute.Key)] = attribute.Value.Emit()
	}
	want := map[string]string{"service.name": "gojellyfin-worker", "k8s.pod.ip": "10.0.0.1", "deployment.environment.name": "home lab"}
	for key, value := range want {
		if values[key] != value {
			t.Errorf("%s = %q, want %q", key, values[key], value)
		}
	}

	if named, _ := describe(env.Tracing{}); named.Attributes()[0].Value.Emit() != serviceName {
		t.Errorf("an unnamed service is %v, want %s", named.Attributes(), serviceName)
	}

	for _, malformed := range []string{"k8s.pod.ip", "=10.0.0.1", "k=%zz"} {
		if _, err := describe(env.Tracing{ResourceAttributes: malformed}); err == nil {
			t.Errorf("%q was accepted", malformed)
		}
	}
}
