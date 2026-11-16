package middleware

import (
	"io"
	"net/http"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

func StreamTracing(t *tracing.Tracing, operationID string, next http.HandlerFunc) http.HandlerFunc {
	if !t.Enabled() {
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := t.StartRequest(r.Context(), r.Header, operationID, r.Method)
		first := &untilFirstByte{ResponseWriter: w, span: span}
		defer first.end(http.StatusOK)

		next(first, r.WithContext(ctx))
	}
}

type untilFirstByte struct {
	http.ResponseWriter

	span  *tracing.Span
	ended bool
}

func (u *untilFirstByte) end(status int) {
	if u.ended {
		return
	}
	u.ended = true

	u.span.Set(tracing.Int("http.response.status_code", status))
	if status >= http.StatusInternalServerError {
		u.span.Failed()
	}
	u.span.End()
}

func (u *untilFirstByte) WriteHeader(status int) {
	u.end(status)
	u.ResponseWriter.WriteHeader(status)
}

func (u *untilFirstByte) Write(b []byte) (int, error) {
	u.end(http.StatusOK)

	return u.ResponseWriter.Write(b)
}

func (u *untilFirstByte) ReadFrom(source io.Reader) (int64, error) {
	u.end(http.StatusOK)
	if from, ok := u.ResponseWriter.(io.ReaderFrom); ok {
		return from.ReadFrom(source)
	}

	return io.Copy(writerOnly{u.ResponseWriter}, source)
}

func (u *untilFirstByte) Flush() {
	u.end(http.StatusOK)
	_ = http.NewResponseController(u.ResponseWriter).Flush()
}

func (u *untilFirstByte) Unwrap() http.ResponseWriter {
	return u.ResponseWriter
}

type writerOnly struct {
	io.Writer
}
