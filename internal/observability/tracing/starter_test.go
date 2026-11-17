package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestStarts(t *testing.T) {
	tracing, recorder := Recorded()
	_, enqueue := tracing.Tracer().Start(context.Background(), "Enqueue")
	parent := trace.ContextWithSpanContext(context.Background(), enqueue.SpanContext())
	start := Starts([]string{"Queued:"}, []string{"Run:"})

	passed := start(parent, tracing.Tracer(), "Queued:Job")
	passed.End()
	if passed.IsRecording() || !passed.SpanContext().Equal(enqueue.SpanContext()) {
		t.Errorf("a queued span recorded or broke the chain: %v", passed.SpanContext())
	}

	start(parent, tracing.Tracer(), "Run:Job").End()
	start(context.Background(), tracing.Tracer(), "Run:Alone").End()
	start(parent, tracing.Tracer(), "Other").End()
	enqueue.End()

	run, _ := ended(recorder, "Run:Job")
	if run.Parent.IsValid() || run.SpanContext.TraceID() == enqueue.SpanContext().TraceID() {
		t.Errorf("the run is in the enqueue's trace, parent %v", run.Parent)
	}
	if len(run.Links) != 1 || run.Links[0].SpanContext.SpanID() != enqueue.SpanContext().SpanID() {
		t.Errorf("links = %v, want the enqueue", run.Links)
	}
	if run.SpanKind != trace.SpanKindConsumer {
		t.Errorf("kind = %v, want consumer", run.SpanKind)
	}

	if alone, _ := ended(recorder, "Run:Alone"); len(alone.Links) != 0 {
		t.Errorf("a run with nothing upstream links %v", alone.Links)
	}
	if other, _ := ended(recorder, "Other"); other.Parent.SpanID() != enqueue.SpanContext().SpanID() {
		t.Errorf("any other span left its parent: %v", other.Parent)
	}
	if names := recorder.Names(); len(names) != 4 {
		t.Errorf("ended = %v, want the enqueue, the two runs and the other", names)
	}
}
