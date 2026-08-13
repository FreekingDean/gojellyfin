package worker

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

const defaultAddr = ":8082"

type Server struct {
	s     *http.Server
	token string
	jobs  chan struct{}
}

func New() *Server {
	addr := os.Getenv("TRANSCODER_ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	worker := &Server{
		s:     &http.Server{Addr: addr},
		token: os.Getenv("TRANSCODER_TOKEN"),
		jobs:  make(chan struct{}, maxJobs()),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if !Available() {
			http.Error(w, "ffmpeg is not installed", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET "+transcode.Path, worker.Handle)
	worker.s.Handler = mux

	return worker
}

// An encode saturates about a core, so running more of them than there are
// cores makes every stream slower without finishing any of them sooner.
func maxJobs() int {
	if jobs, err := strconv.Atoi(os.Getenv("TRANSCODER_JOBS")); err == nil && jobs > 0 {
		return jobs
	}

	return runtime.NumCPU()
}

// The spec names a file for ffmpeg to open, so an unauthenticated caller could
// read anything the worker can. The token is what keeps this to the API.
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	spec, err := transcode.SpecFromQuery(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The pool reads any non-200 as "try the next worker", so refusing here is
	// what turns its round robin into least loaded.
	select {
	case s.jobs <- struct{}{}:
	default:
		http.Error(w, "every job is taken", http.StatusServiceUnavailable)
		return
	}
	defer func() { <-s.jobs }()

	output, err := start(r.Context(), spec)
	if err != nil {
		log.Printf("failed to transcode %s to %s: %v", spec.Path, spec.Container, err)
		http.Error(w, "failed to transcode", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := output.Close(); err != nil {
			log.Printf("transcode of %s ended: %v", spec.Path, err)
		}
	}()

	w.Header().Set("Content-Type", transcode.ContentType(spec.Container))
	w.WriteHeader(http.StatusOK)

	if _, err := transcode.Relay(w, output); err != nil {
		log.Printf("transcode of %s stopped: %v", spec.Path, err)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}

	return subtle.ConstantTimeCompare([]byte(r.Header.Get(transcode.TokenHeader)), []byte(s.token)) == 1
}

func (s *Server) Handler() http.Handler {
	return s.s.Handler
}

func (s *Server) ListenAndServe() {
	log.Printf("transcoder listening on %s", s.s.Addr)
	if err := s.s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("ListenAndServe error: %v", err)
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	if err := s.s.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}
