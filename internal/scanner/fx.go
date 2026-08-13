package scanner

import (
	"context"
	"log"

	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/tasks"
)

var Module = fx.Module(
	"scanner",
	fx.Provide(
		New,
	),
	fx.Invoke(
		useScanner,
		run,
	),
)

func useScanner(registry *tasks.Registry, s *Scanner) error {
	return registry.UseRunner(tasks.LibraryScanID, s.Scan)
}

func run(lc fx.Lifecycle, s *Scanner) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := s.Scan(context.Background()); err != nil {
					log.Printf("initial scan: %v", err)
				}
			}()

			return nil
		},
	})
}
