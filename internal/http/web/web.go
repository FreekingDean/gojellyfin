package web

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

const defaultRoot = "/usr/share/jellyfin/web"

// Unset falls back to the path the jellyfin-web package installs to; set and
// empty turns serving off for an operator hosting the client elsewhere.
func Root() string {
	if root, ok := os.LookupEnv("JELLYFIN_WEB_ROOT"); ok {
		return root
	}

	return defaultRoot
}

type Handler struct {
	root  string
	index string
}

func New() *Handler {
	root := Root()
	if root == "" {
		return &Handler{}
	}

	index := filepath.Join(root, "index.html")
	if _, err := os.Stat(index); err != nil {
		log.Printf("no web client at %s, serving the API only", root)
		return &Handler{}
	}

	return &Handler{root: root, index: index}
}

func (h *Handler) Register(m *mux.Mux) {
	if h.root == "" {
		return
	}

	m.HandleFunc("GET /web/*", h.serve)
	m.HandleFunc("GET /web", h.redirect)
	m.HandleFunc("GET /", h.redirect)
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/web/", http.StatusFound)
}

// http.ServeFile is avoided because it answers /web/index.html with a redirect
// to ./, and index.html is the entry point clients ask for by name.
func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	name := h.file(r.PathValue("rest"))

	file, err := os.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, name, info.ModTime(), file)
}

func (h *Handler) file(rest string) string {
	name := filepath.Join(h.root, filepath.Clean("/"+rest))
	if info, err := os.Stat(name); err != nil || info.IsDir() {
		return h.index
	}

	return name
}
