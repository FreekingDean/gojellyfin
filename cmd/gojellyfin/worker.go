package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/observability"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

func workerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Run background workflows",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			fx.New(
				observability.Module,
				store.Module,
				fx.Provide(
					items.New,
					libraries.New,
					filesystem.New,
				),
				scanner.Module,
				jobs.Module,
				jobs.WorkerModule,
			).Run()
		},
	}
}
