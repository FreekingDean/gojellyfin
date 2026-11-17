package observability

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

var Module = fx.Module(
	"observability",
	tracing.Module,
)
