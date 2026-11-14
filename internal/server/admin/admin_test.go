package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/sso"
)

func serve(t *testing.T, files fstest.MapFS, cfg env.Config, path string) *httptest.ResponseRecorder {
	t.Helper()

	h, err := newHandler(files, sso.New(cfg, nil))
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}
	m := mux.New()
	h.Register(m)

	recorder := httptest.NewRecorder()
	m.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	return recorder
}

func built() fstest.MapFS {
	return fstest.MapFS{
		"index.html":        {Data: []byte(`<meta name="gojellyfin-admin" content="{{.}}"><div id="app"></div>`)},
		"assets/index-a.js": {Data: []byte(`console.log("admin")`)},
	}
}

func TestHandler_ServesTheAppOnEveryRoute(t *testing.T) {
	for _, path := range []string{"/admin", "/admin/", "/Admin/users", "/admin/libraries/new-movies?mock", "/admin/index.html"} {
		t.Run(path, func(t *testing.T) {
			recorder := serve(t, built(), env.Config{}, path)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			want := `content="{&#34;ServerId&#34;:&#34;` + config.ServerID + `&#34;,&#34;SingleSignOn&#34;:false}"`
			if body := recorder.Body.String(); !strings.Contains(body, want) {
				t.Errorf("body = %s, want it to contain %s", body, want)
			}
		})
	}
}

func TestHandler_SaysWhenSingleSignOnIsOn(t *testing.T) {
	cfg := env.Config{OIDC: env.OIDC{IssuerURL: "https://id.example"}}

	recorder := serve(t, built(), cfg, "/admin")

	if body := recorder.Body.String(); !strings.Contains(body, `SingleSignOn&#34;:true`) {
		t.Errorf("body = %s, want single sign-on on", body)
	}
}

func TestHandler_ServesAssets(t *testing.T) {
	recorder := serve(t, built(), env.Config{}, "/admin/assets/index-a.js")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable", got)
	}
	if got := recorder.Body.String(); got != `console.log("admin")` {
		t.Errorf("body = %q", got)
	}
}

func TestHandler_AMissingAssetIsNotTheApp(t *testing.T) {
	for _, path := range []string{"/admin/assets/index-b.js", "/admin/assets/../../secret"} {
		recorder := serve(t, built(), env.Config{}, path)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, recorder.Code)
		}
	}
}

func TestHandler_SaysWhenTheAppWasNotBuilt(t *testing.T) {
	recorder := serve(t, fstest.MapFS{".gitkeep": {}}, env.Config{}, "/admin")

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "make admin") {
		t.Errorf("body = %q, want it to name make admin", body)
	}
}

func TestNew_ReadsTheEmbeddedApp(t *testing.T) {
	if _, err := New(sso.New(env.Config{}, nil)); err != nil {
		t.Fatalf("New: %v", err)
	}
}
