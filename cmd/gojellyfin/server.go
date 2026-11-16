package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/apikeys"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/cards"
	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/health"
	"github.com/FreekingDean/gojellyfin/internal/http"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/metadata"
	"github.com/FreekingDean/gojellyfin/internal/notify"
	"github.com/FreekingDean/gojellyfin/internal/observability"
	"github.com/FreekingDean/gojellyfin/internal/playlists"
	"github.com/FreekingDean/gojellyfin/internal/probe"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/requests"
	"github.com/FreekingDean/gojellyfin/internal/segments"
	"github.com/FreekingDean/gojellyfin/internal/server"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/sso"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
	"github.com/FreekingDean/gojellyfin/internal/system"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var serverModules = fx.Options(
	env.Module,
	observability.Module,
	store.Module,

	activity.Module,
	apikeys.Module,
	auth.Module,
	cards.Module,
	config.Module,
	discover.Module,
	displaypreferences.Module,
	ffmpeg.Module,
	filesystem.Module,
	health.Module,
	items.Module,
	libraries.Module,
	localization.Module,
	metadata.Module,
	notify.Module,
	playlists.Module,
	probe.Module,
	segments.Module,
	quickconnect.Module,
	reports.Module,
	requests.Module,
	shelves.Module,
	channels.Module,
	sources.Module,
	sessions.Module,
	sso.Module,
	syncplay.Module,
	system.Module,
	jobs.Module,
	users.Module,

	server.Module,
	http.Module,
)

func serverCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Serve the Jellyfin API",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			fx.New(serverModules).Run()
		},
	}
}
