package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/http"
	"github.com/FreekingDean/gojellyfin/internal/observability"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/system"
)

func serverCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Serve the Jellyfin API",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			fx.New(
				observability.Module,
				store.Module,
				system.Module,
				scanner.Module,
				server.Module,
				http.Module,
			).Run()
		},
	}
}
