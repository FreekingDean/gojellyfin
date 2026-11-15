package syncplay

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/notify"
)

var Module = fx.Module(
	"server/syncplay",
	fx.Provide(
		New,
		func(notifier *notify.Service) Publisher { return notifier },
	),
)
