package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store/migrations"
)

func migrateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply the versioned migrations",
		Long:  "Applies the versioned migrations in internal/store/migrations, which nothing applies at startup.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			atlas, err := exec.LookPath("atlas")
			if err != nil {
				return fmt.Errorf("atlas is not on PATH: this command drives the atlas CLI, install it from https://atlasgo.io/docs#installation")
			}

			dir, err := unpack()
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()

			config, err := env.Load()
			if err != nil {
				return err
			}

			apply := exec.CommandContext(cmd.Context(), atlas, "migrate", "apply", "--dir", "file://"+dir, "--url", config.DatabaseURL)
			apply.Stdout = os.Stdout
			apply.Stderr = os.Stderr
			if err := apply.Run(); err != nil {
				return fmt.Errorf("failed to apply migrations: %w", err)
			}

			return nil
		},
	}
}

// The migrations ship inside the binary so a deployed image cannot drift from
// the code that expects the schema.
func unpack() (string, error) {
	dir, err := os.MkdirTemp("", "gojellyfin-migrations")
	if err != nil {
		return "", fmt.Errorf("failed to unpack migrations: %w", err)
	}

	if err := os.CopyFS(dir, migrations.FS); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("failed to unpack migrations: %w", err)
	}

	return dir, nil
}
