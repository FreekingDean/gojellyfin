package request

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

type signOn bool

func (s signOn) Enabled() bool { return bool(s) }

func TestHandler_Page(t *testing.T) {
	for _, tc := range []struct {
		name   string
		signOn bool
	}{
		{"without single sign-on", false},
		{"with single sign-on", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := mux.New()
			(&Handler{signOn: signOn(tc.signOn)}).Register(m)

			recorder := httptest.NewRecorder()
			m.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/request?q=zebracorn", nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}

			body := recorder.Body.String()
			for _, want := range []string{`var serverId = "` + config.ServerID + `";`, "/GoJellyfin/MyRequests", "/UserFavoriteItems/", "jellyfin_credentials"} {
				if !strings.Contains(body, want) {
					t.Errorf("the page is missing %s", want)
				}
			}
			if got := strings.Contains(body, "/sso/oidc/start?redirect=%2Frequest"); got != tc.signOn {
				t.Errorf("single sign-on button drawn = %v, want %v", got, tc.signOn)
			}
			if strings.Contains(body, "zebracorn") {
				t.Errorf("the page echoes the query it was asked with")
			}
		})
	}
}
