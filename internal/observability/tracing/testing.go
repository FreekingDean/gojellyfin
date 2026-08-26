package tracing

import (
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

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

func (r *Recorder) Values() map[string]string {
	values := make(map[string]string)
	for _, span := range r.recorder.Ended() {
		for _, attribute := range span.Attributes() {
			values[string(attribute.Key)] = attribute.Value.Emit()
		}
	}

	return values
}
