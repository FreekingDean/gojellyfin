package pprof

import (
	"context"
	"errors"
	"log"
	"net/http"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"pprof",
	fx.Provide(New),
	fx.Invoke(run),
)

func run(lc fx.Lifecycle, s *Server) {
	if s.server == nil {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			log.Printf("pprof listening on %s", s.server.Addr)
			go func() {
				if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Printf("pprof: %v", err)
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return s.server.Shutdown(ctx)
		},
	})
}
