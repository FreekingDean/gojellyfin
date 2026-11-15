package syncplay

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/notify"
	"github.com/FreekingDean/gojellyfin/internal/server/socket"
)

var Module = fx.Module(
	"server/syncplay",
	fx.Provide(
		New,
		func(notifier *notify.Service) Publisher { return notifier },
	),
	fx.Invoke(resync),
)

func resync(server *Server, sockets *socket.Socket, notifier *notify.Service) {
	sockets.OnConnect(func(ctx context.Context, sessionID uuid.UUID) {
		server.Resync(ctx, []uuid.UUID{sessionID})
	})
	notifier.OnReconnect(func(ctx context.Context) {
		server.Resync(ctx, sockets.Sessions())
	})
}
