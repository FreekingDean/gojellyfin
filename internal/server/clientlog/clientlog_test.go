package clientlog

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestServer_LogFile(t *testing.T) {
	var written bytes.Buffer
	log.SetOutput(&written)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	body := api.LogFileTextRequestBody("java.lang.IllegalStateException: boom")

	response, err := New().LogFile(context.Background(), api.LogFileRequestObject{Body: &body})
	if err != nil {
		t.Fatalf("failed to take the document: %v", err)
	}

	result, ok := response.(api.LogFile200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.LogFile200JSONResponse", response)
	}
	if result.FileName == nil || *result.FileName == "" {
		t.Fatalf("FileName = %v, want a name", result.FileName)
	}
	if !strings.Contains(written.String(), "IllegalStateException: boom") {
		t.Fatalf("log = %q, want the document in it", written.String())
	}
}

func TestDocument(t *testing.T) {
	long := api.LogFileTextRequestBody(strings.Repeat("a", logged+10))

	if got := document(nil); got != "" {
		t.Fatalf("document(nil) = %q, want empty", got)
	}
	if got := len(document(&long)); got != logged {
		t.Fatalf("len = %d, want %d", got, logged)
	}
}
