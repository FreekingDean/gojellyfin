package request

import (
	_ "embed"
	"html/template"
	"log"
	"net/http"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/sso"
)

const Path = "/request"

//go:embed page.html
var markup string

var page = template.Must(template.New("request").Parse(markup))

type SignOn interface {
	Enabled() bool
}

type Handler struct {
	signOn SignOn
}

func New(signOn *sso.Service) *Handler {
	return &Handler{signOn: signOn}
}

func (h *Handler) Register(m *mux.Mux) {
	m.HandleFunc("GET "+Path, h.Page)
}

type settings struct {
	ServerID     string
	SingleSignOn bool
}

func (h *Handler) Page(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	if err := page.Execute(w, settings{ServerID: config.ServerID, SingleSignOn: h.signOn.Enabled()}); err != nil {
		log.Printf("request: failed to write the page: %v", err)
	}
}
