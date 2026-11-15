package webhook

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/sources"
)

const maxBody = 1 << 20

type Sources interface {
	Authenticate(ctx context.Context, id uuid.UUID, username, password string) (sources.Source, error)
}

type Handler struct {
	sources Sources
	jobs    jobs.Enqueuer
}

func New(downloaders *sources.Service, background *jobs.Service) *Handler {
	return &Handler{sources: downloaders, jobs: background}
}

func (h *Handler) Register(m *mux.Mux) {
	m.HandleFunc("POST "+sources.WebhookPath+"/{sourceId}", h.Serve)
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("sourceId"))
	if err != nil {
		http.NotFound(w, r)

		return
	}

	username, password, _ := r.BasicAuth()
	source, err := h.sources.Authenticate(r.Context(), id, username, password)
	if errors.Is(err, sources.ErrUnauthorized) {
		w.Header().Set("WWW-Authenticate", `Basic realm="gojellyfin"`)
		http.Error(w, err.Error(), http.StatusUnauthorized)

		return
	}
	if err != nil {
		log.Printf("webhook: failed to authenticate %s: %v", id, err)
		http.Error(w, "failed to read the source", http.StatusInternalServerError)

		return
	}

	hooked, refreshes, err := sources.Heard(source, http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}
	if !refreshes {
		w.WriteHeader(http.StatusNoContent)

		return
	}

	if err := h.jobs.Enqueue(r.Context(), jobs.RefreshSourceTitle, hooked.Params(source.ID)...); err != nil {
		log.Printf("webhook: failed to enqueue a refresh from %s: %v", source.Name, err)
		http.Error(w, "background work is unavailable", http.StatusServiceUnavailable)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}
