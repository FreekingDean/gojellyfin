package transcode

import (
	"io"
	"net/http"
)

func Relay(w http.ResponseWriter, output io.Reader) (int64, error) {
	return io.Copy(&flushing{w: w}, output)
}

func Flushed(w http.ResponseWriter) io.Writer {
	return &flushing{w: w}
}

type flushing struct {
	w http.ResponseWriter
}

func (f *flushing) Write(b []byte) (int, error) {
	n, err := f.w.Write(b)
	if err == nil {
		_ = http.NewResponseController(f.w).Flush()
	}

	return n, err
}
