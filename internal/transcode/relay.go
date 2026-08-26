package transcode

import (
	"io"
	"net/http"
)

func Relay(w http.ResponseWriter, output io.Reader) (int64, error) {
	return io.Copy(&flushing{w: w}, output)
}

type flushing struct {
	w http.ResponseWriter
}

func (f *flushing) Write(b []byte) (int, error) {
	n, err := f.w.Write(b)
	if flusher, ok := f.w.(http.Flusher); ok {
		flusher.Flush()
	}

	return n, err
}
