package admin

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/sso"
)

const (
	Path = "/admin"

	indexName   = "index.html"
	assetPrefix = "assets/"
	notBuilt    = "The admin app was not built into this binary. Run make admin and build again."
)

//go:embed all:dist
var embedded embed.FS

type settings struct {
	ServerId     string
	SingleSignOn bool
}

type Handler struct {
	files fs.FS
	index *template.Template
	sso   *sso.Service
}

func New(signOn *sso.Service) (*Handler, error) {
	files, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}

	return newHandler(files, signOn)
}

func newHandler(files fs.FS, signOn *sso.Service) (*Handler, error) {
	h := &Handler{files: files, sso: signOn}

	markup, err := fs.ReadFile(files, indexName)
	if err != nil {
		return h, nil
	}
	h.index, err = template.New(indexName).Parse(string(markup))
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (h *Handler) Register(m *mux.Mux) {
	m.HandleFunc("GET "+Path, h.Serve)
	m.HandleFunc("GET "+Path+"/*", h.Serve)
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	if h.index == nil {
		http.Error(w, notBuilt, http.StatusServiceUnavailable)
		return
	}

	name := r.PathValue("rest")
	if h.isFile(name) {
		if strings.HasPrefix(name, assetPrefix) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeFileFS(w, r, h.files, name)
		return
	}
	if strings.HasPrefix(name, assetPrefix) {
		http.NotFound(w, r)
		return
	}

	h.page(w)
}

func (h *Handler) isFile(name string) bool {
	if name == "" || name == indexName || !fs.ValidPath(name) {
		return false
	}
	info, err := fs.Stat(h.files, name)

	return err == nil && !info.IsDir()
}

func (h *Handler) page(w http.ResponseWriter) {
	content, err := json.Marshal(settings{ServerId: config.ServerID, SingleSignOn: h.sso.Enabled()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	if err := h.index.Execute(w, string(content)); err != nil {
		log.Printf("admin: failed to write the page: %v", err)
	}
}
