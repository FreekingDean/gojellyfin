package syncplay

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/notify"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
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
	fx.Invoke(func(lifecycle fx.Lifecycle, sweeper *Sweeper) { lifecycle.Append(fx.StartStopHook(sweeper.Start, sweeper.Stop)) }),
	fx.Invoke(func(sessions *sessions.Service, service *Service) { sessions.OnEnding(service.SessionsEnding) }),
)
