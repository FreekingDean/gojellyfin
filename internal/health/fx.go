package health

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/metadata"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/sso"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

var Module = fx.Module(
	"health",
	fx.Provide(provide),
)

func provide(
	client *store.Client,
	background *jobs.Service,
	identify *metadata.Service,
	signOn *sso.Service,
	encoder *transcode.Encoder,
	downloaders *sources.Service,
	spans *tracing.Tracing,
	catalogue *items.Service,
) *Service {
	return New(client, background, identify, signOn, encoder, downloaders, spans, catalogue)
}
