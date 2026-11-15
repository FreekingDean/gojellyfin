package http

import (
	"net/http"

	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

const clientPath = "/web/"

func registerRoot(m *mux.Mux) {
	for _, pattern := range []string{"GET /", "HEAD /"} {
		m.HandleFunc(pattern, redirectToClient)
	}
}

func redirectToClient(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, clientPath, http.StatusFound)
}
