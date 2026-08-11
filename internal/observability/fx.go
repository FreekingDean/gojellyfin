package observability

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/observability/log"
)

var Module = fx.Module(
	"observability",
	log.Module,
)
