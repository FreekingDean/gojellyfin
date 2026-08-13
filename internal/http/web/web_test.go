package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

func root(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.html": "<html>client</html>",
		"main.js":    "console.log(1)",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("JELLYFIN_WEB_ROOT", dir)

	return dir
}

func serve(t *testing.T, m *mux.Mux, path string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest("GET", path, nil))

	return w
}

func TestServesTheClient(t *testing.T) {
	root(t)
	m := mux.New()
	New().Register(m)

	tests := []struct {
		path   string
		status int
		body   string
	}{
		{"/", http.StatusFound, ""},
		{"/web", http.StatusFound, ""},
		{"/web/", http.StatusOK, "<html>client</html>"},
		{"/web/index.html", http.StatusOK, "<html>client</html>"},
		{"/web/main.js", http.StatusOK, "console.log(1)"},
		{"/web/movies/list", http.StatusOK, "<html>client</html>"},
	}

	for _, test := range tests {
		w := serve(t, m, test.path)
		if w.Code != test.status {
			t.Errorf("GET %s = %d, want %d", test.path, w.Code, test.status)
		}
		if test.body != "" && w.Body.String() != test.body {
			t.Errorf("GET %s body = %q, want %q", test.path, w.Body.String(), test.body)
		}
	}

	if location := serve(t, m, "/").Header().Get("Location"); location != "/web/" {
		t.Errorf("GET / redirected to %q, want %q", location, "/web/")
	}
}

func TestAssetsKeepTheirContentType(t *testing.T) {
	root(t)
	m := mux.New()
	New().Register(m)

	got := serve(t, m, "/web/main.js").Header().Get("Content-Type")
	if got != "text/javascript; charset=utf-8" {
		t.Errorf("main.js served as %q", got)
	}
}

func TestStaysInsideTheRoot(t *testing.T) {
	dir := root(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "secret"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := mux.New()
	New().Register(m)

	if body := serve(t, m, "/web/../secret").Body.String(); body == "no" {
		t.Error("a traversal escaped the web root")
	}
}

// The mux matches in registration order, so an API route under /web only
// survives if the catch-all is registered after it.
func TestApiRoutesUnderWebWin(t *testing.T) {
	root(t)
	m := mux.New()

	api := false
	m.HandleFunc("GET /web/ConfigurationPages", func(http.ResponseWriter, *http.Request) { api = true })
	New().Register(m)

	serve(t, m, "/web/ConfigurationPages")
	if !api {
		t.Error("the web catch-all swallowed /web/ConfigurationPages")
	}
}

func TestAMissingClientRegistersNothing(t *testing.T) {
	t.Setenv("JELLYFIN_WEB_ROOT", filepath.Join(t.TempDir(), "absent"))
	m := mux.New()
	New().Register(m)

	if w := serve(t, m, "/web/index.html"); w.Code != http.StatusNotFound {
		t.Errorf("GET /web/index.html = %d, want 404 with no client on disk", w.Code)
	}
}

func TestAnEmptyRootTurnsServingOff(t *testing.T) {
	root(t)
	t.Setenv("JELLYFIN_WEB_ROOT", "")
	m := mux.New()
	New().Register(m)

	if w := serve(t, m, "/"); w.Code != http.StatusNotFound {
		t.Errorf("GET / = %d, want 404 with serving off", w.Code)
	}
}
