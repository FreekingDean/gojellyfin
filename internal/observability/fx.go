package observability

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/observability/log"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

var Module = fx.Module(
	"observability",
	log.Module,
	tracing.Module,
)
