package report

import (
	_ "embed"
	"html/template"
	"log"
	"net/http"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

const Path = "/report"

//go:embed page.html
var markup string

var page = template.Must(template.New("report").Parse(markup))

type Handler struct{}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) Register(m *mux.Mux) {
	m.HandleFunc("GET "+Path, h.Page)
}

func (h *Handler) Page(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	if err := page.Execute(w, struct{ ServerID string }{ServerID: config.ServerID}); err != nil {
		log.Printf("report: failed to write the page: %v", err)
	}
}
