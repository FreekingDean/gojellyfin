package transcode

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"transcode",
	fx.Provide(
		NewPool,
	),
)
