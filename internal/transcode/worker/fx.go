package worker

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"transcodeworker",
	fx.Provide(
		New,
	),
	fx.Invoke(
		run,
	),
)

func run(lc fx.Lifecycle, s *Server) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go s.ListenAndServe()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.Shutdown(ctx)

			return nil
		},
	})
}
