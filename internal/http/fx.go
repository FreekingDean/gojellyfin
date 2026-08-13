package http

import (
	"context"

	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/server/socket"
	"github.com/FreekingDean/gojellyfin/internal/server/stream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

var Module = fx.Module(
	"http",
	mux.Module,
	transcode.Module,
	fx.Provide(
		middleware.NewAuth,
		socket.New,
		func(pool *transcode.Pool) stream.Transcoder { return pool },
		stream.New,
		New,
	),
	fx.Invoke(
		Register,
		run,
	),
)

func run(lc fx.Lifecycle, s *Server) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go s.ListenAndServe()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.Shutdown(ctx)

			return nil
		},
	})
}
