package link_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/server/link"
)

func TestRedirect(t *testing.T) {
	m := mux.New()
	for _, pattern := range link.Routes {
		m.HandleFunc(pattern, link.Redirect)
	}

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{"bare", http.MethodGet, "/link", "/web/#/quickconnect"},
		{"trailing slash", http.MethodGet, "/link/", "/web/#/quickconnect"},
		{"any case", http.MethodGet, "/LINK", "/web/#/quickconnect"},
		{"code in the path", http.MethodGet, "/link/123456", "/web/#/quickconnect?code=123456"},
		{"code in the query", http.MethodGet, "/link?code=654321", "/web/#/quickconnect?code=654321"},
		{"code typed with a space", http.MethodGet, "/link/123%20456", "/web/#/quickconnect?code=123456"},
		{"code typed with a dash", http.MethodGet, "/link/123-456", "/web/#/quickconnect?code=123456"},
		{"nothing numeric", http.MethodGet, "/link/hello", "/web/#/quickconnect"},
		{"markup", http.MethodGet, "/link?code=%22%3E%3Cscript%3E1", "/web/#/quickconnect?code=1"},
		{"head", http.MethodHead, "/link/123456", "/web/#/quickconnect?code=123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			m.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))

			if recorder.Code != http.StatusFound {
				t.Fatalf("want 302, got %d", recorder.Code)
			}
			if got := recorder.Header().Get("Location"); got != tt.want {
				t.Errorf("want Location %q, got %q", tt.want, got)
			}

			cookies := recorder.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("want one cookie, got %d", len(cookies))
			}
			returnTo, err := url.QueryUnescape(cookies[0].Value)
			if err != nil {
				t.Fatalf("the return cookie does not unescape: %v", err)
			}
			if got := "/web/#" + returnTo; got != tt.want {
				t.Errorf("want the sign-in to return to %q, got %q", tt.want, got)
			}
			if cookie := cookies[0]; cookie.Name != "gojellyfin_sso_return" || cookie.Path != "/sso/oidc" || cookie.MaxAge != 600 || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("want the return cookie the sign-in reads, got %+v", cookie)
			}
		})
	}
}
