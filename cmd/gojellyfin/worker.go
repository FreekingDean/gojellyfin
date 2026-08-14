package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/observability"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/tmdb"
)

var workerModules = fx.Options(
	env.Module,
	observability.Module,
	store.Module,
	fx.Provide(
		items.New,
		libraries.New,
		filesystem.New,
	),
	scanner.Module,
	tmdb.Module,
	jobs.Module,
	jobs.WorkerModule,
)

func workerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Run background workflows",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			fx.New(workerModules).Run()
		},
	}
}
