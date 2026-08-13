package main

import (
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/observability"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/worker"
)

func workerCommand() *cobra.Command {
	options := worker.Options{}

	command := &cobra.Command{
		Use:   "worker",
		Short: "Run queued jobs",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			fx.New(
				observability.Module,
				store.Module,
				fx.Provide(
					jobs.New,
					items.New,
					libraries.New,
					filesystem.New,
				),
				fx.Supply(options),
				scanner.Module,
				worker.Module,
			).Run()
		},
	}
	command.Flags().IntVar(&options.Concurrency, "concurrency", 1, "how many jobs to run at once")
	command.Flags().DurationVar(&options.Lease, "lease", time.Minute, "how long a leased job is held before the queue reclaims it")

	return command
}
