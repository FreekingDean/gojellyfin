package fx

import (
	"context"

	"go.uber.org/fx"
)

type StartStop interface {
	Start() error
	Stop() error
}

func InvokeStartStop[s StartStop]() fx.Option {
	return fx.Invoke(
		func(lifecycle fx.Lifecycle, s s) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return s.Start()
				},
				OnStop: func(ctx context.Context) error {
					return s.Stop()
				},
			})
		},
	)
}
