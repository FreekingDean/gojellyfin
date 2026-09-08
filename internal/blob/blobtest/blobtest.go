package blobtest

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

const (
	Bucket        = "artwork"
	streamingBody = "STREAMING-AWS4-HMAC-SHA256-PAYLOAD"
)

type object struct {
	body        []byte
	contentType string
}

type fake struct {
	mutex   sync.Mutex
	objects map[string]object
}

func Server(t *testing.T) env.ObjectStore {
	t.Helper()

	server := httptest.NewServer(&fake{objects: map[string]object{}})
	t.Cleanup(server.Close)

	return env.ObjectStore{
		Endpoint:  server.URL,
		Bucket:    Bucket,
		Region:    "us-east-1",
		AccessKey: "key",
		SecretKey: "secret",
	}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/"+Bucket+"/")

	f.mutex.Lock()
	defer f.mutex.Unlock()

	switch r.Method {
	case http.MethodPut:
		f.put(w, r, key)
	case http.MethodGet, http.MethodHead:
		f.get(w, r, key)
	case http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *fake) put(w http.ResponseWriter, r *http.Request, key string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	if r.Header.Get("X-Amz-Content-Sha256") == streamingBody {
		body = unchunked(body)
	}

	f.objects[key] = object{body: body, contentType: r.Header.Get("Content-Type")}
	w.Header().Set("ETag", `"fake"`)
	w.WriteHeader(http.StatusOK)
}

func (f *fake) get(w http.ResponseWriter, r *http.Request, key string) {
	found, ok := f.objects[key]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `<Error><Code>NoSuchKey</Code><Key>%s</Key></Error>`, key)

		return
	}

	w.Header().Set("Content-Type", found.contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(found.body)))
	w.Header().Set("ETag", `"fake"`)
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodGet {
		_, _ = w.Write(found.body)
	}
}

func unchunked(body []byte) []byte {
	decoded := []byte{}
	for {
		newline := bytes.Index(body, []byte("\r\n"))
		if newline < 0 {
			return decoded
		}

		header := body[:newline]
		if semicolon := bytes.IndexByte(header, ';'); semicolon >= 0 {
			header = header[:semicolon]
		}

		size, err := strconv.ParseInt(string(header), 16, 64)
		if err != nil || size == 0 {
			return decoded
		}

		start := newline + 2
		decoded = append(decoded, body[start:start+int(size)]...)
		body = body[start+int(size)+2:]
	}
}
