package scanner

import (
	"context"
	"log"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"scanner",
	fx.Provide(
		New,
	),
	fx.Invoke(
		run,
	),
)

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
