package pprof

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnsetAddressServesNothing(t *testing.T) {
	t.Setenv("PPROF_ADDR", "")

	if New().server != nil {
		t.Error("an unset PPROF_ADDR built a listener")
	}
}

func TestGoroutineDumpNamesTheStacks(t *testing.T) {
	t.Setenv("PPROF_ADDR", "127.0.0.1:0")

	server := httptest.NewServer(New().server.Handler)
	defer server.Close()

	response, err := http.Get(server.URL + "/debug/pprof/goroutine?debug=2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	// The whole point of debug=2 is the stack, not the count.
	if !strings.Contains(string(body), "goroutine ") {
		t.Errorf("the dump carries no stacks: %.120q", body)
	}
}
