package scanner

import (
	"context"
	"log"

	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/worker"
)

var Module = fx.Module(
	"scanner",
	fx.Provide(
		New,
	),
	fx.Invoke(
		register,
	),
)

func register(lc fx.Lifecycle, w *worker.Worker, s *Scanner, service *jobs.Service) {
	w.Handle(jobs.LibraryScanKind, func(ctx context.Context, _ *jobs.Job, report jobs.Reporter) error {
		return s.Scan(ctx, report)
	})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if _, err := service.Start(ctx, jobs.LibraryScanKind); err != nil {
				log.Printf("initial scan: %v", err)
			}

			return nil
		},
	})
}
