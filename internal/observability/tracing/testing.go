package tracing

import (
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// Recorded is a Tracing that keeps its spans in memory, so a caller can assert
// on what it produced without a collector and without reaching for otel.
func Recorded() (*Tracing, *Recorder) {
	recorder := tracetest.NewSpanRecorder()

	return &Tracing{
		provider:   sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
		propagator: propagation.TraceContext{},
	}, &Recorder{recorder: recorder}
}

type Recorder struct {
	recorder *tracetest.SpanRecorder
}

func (r *Recorder) Names() []string {
	names := make([]string, 0)
	for _, span := range r.recorder.Ended() {
		names = append(names, span.Name())
	}

	return names
}

// Every attribute value recorded, which is what a test asserting that nothing
// the client sent reached a span has to look through.
func (r *Recorder) Values() map[string]string {
	values := make(map[string]string)
	for _, span := range r.recorder.Ended() {
		for _, attribute := range span.Attributes() {
			values[string(attribute.Key)] = attribute.Value.Emit()
		}
	}

	return values
}
