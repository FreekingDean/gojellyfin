package syncplay

import (
	"github.com/FreekingDean/gojellyfin/internal/fx"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/notify"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var Module = fx.Module(
	"syncplay",
	fx.Provide(
		New,
		NewSweeper,
		func(service *items.Service) Library { return service },
		func(service *users.Service) Policies { return service },
		func(notifier *notify.Service) Publisher { return notifier },
	),
	fx.InvokeStartStop[*Sweeper](),
)
