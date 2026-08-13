package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/observability"
	"github.com/FreekingDean/gojellyfin/internal/transcode/worker"
)

func transcoderCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "transcoder",
		Short: "Run ffmpeg on behalf of the API",
		Long: "Runs ffmpeg on behalf of the API, which proxies the output back to the client. " +
			"Deployed as its own container so a runaway encode cannot starve the request path; " +
			"it needs the media mounted at the same paths the API sees and no database at all.",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if !worker.Available() {
				return fmt.Errorf("ffmpeg is not on PATH")
			}

			fx.New(
				observability.Module,
				worker.Module,
			).Run()

			return nil
		},
	}
}
