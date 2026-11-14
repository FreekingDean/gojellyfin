package report

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

func TestHandler_Page(t *testing.T) {
	m := mux.New()
	New().Register(m)

	recorder := httptest.NewRecorder()
	m.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/report?item=abc", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	body := recorder.Body.String()
	for _, want := range []string{`var serverId = "` + config.ServerID + `";`, "/GoJellyfin/Reports", "jellyfin_credentials"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page is missing %s", want)
		}
	}
	if strings.Contains(body, "abc") {
		t.Errorf("the page echoes the query it was asked with")
	}
}
