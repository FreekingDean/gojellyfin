package observability

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

var Module = fx.Module(
	"observability",
	tracing.Module,
)
