package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/apikeys"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/http"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/observability"
	"github.com/FreekingDean/gojellyfin/internal/playlists"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/system"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var serverModules = fx.Options(
	observability.Module,
	store.Module,

	activity.Module,
	apikeys.Module,
	auth.Module,
	config.Module,
	displaypreferences.Module,
	filesystem.Module,
	items.Module,
	libraries.Module,
	localization.Module,
	playlists.Module,
	scanner.Module,
	sessions.Module,
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
