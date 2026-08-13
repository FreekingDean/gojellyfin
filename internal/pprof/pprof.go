package pprof

import (
	"net/http"
	httppprof "net/http/pprof"
	"os"
	"time"
)

type Server struct {
	server *http.Server
}

// Its own listener rather than a route on the API's: the API's is published
// through the gateway, and a heap profile carries live data while a goroutine
// dump names every symbol in the binary. Nothing here logs either, so a dump is
// still reachable when whatever is wedged is holding the logger.
func New() *Server {
	address := os.Getenv("PPROF_ADDR")
	if address == "" {
		return &Server{}
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/debug/pprof/", httppprof.Index)
	handler.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
	handler.HandleFunc("/debug/pprof/profile", httppprof.Profile)
	handler.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)
	handler.HandleFunc("/debug/pprof/trace", httppprof.Trace)

	return &Server{
		server: &http.Server{
			Addr:    address,
			Handler: handler,
			// No write timeout: a CPU profile takes thirty seconds by default
			// and a trace as long as it is asked for.
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}
