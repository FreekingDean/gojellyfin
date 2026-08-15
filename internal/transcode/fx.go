package transcode

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

var Module = fx.Module(
	"transcode",
	fx.Provide(New),
)

func New(config env.Config) *Encoder {
	return NewEncoder(config.Transcoder.Jobs, config.Transcoder.StallTimeout)
}
