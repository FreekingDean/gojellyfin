package store

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"store",
	fx.Provide(
		New,
	),
	fx.Invoke(
		run,
	),
)

func run(lc fx.Lifecycle, client *Client) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return client.Schema.Create(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})
}
