// Applies the versioned migrations in internal/store/migrations, which nothing
// applies at startup.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/FreekingDean/gojellyfin/internal/store/migrations"
)

const defaultDatabaseURL = "postgres://localhost:5432/gojellyfin_development?sslmode=disable"

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	atlas, err := exec.LookPath("atlas")
	if err != nil {
		return fmt.Errorf("atlas is not on PATH: this command drives the atlas CLI, install it from https://atlasgo.io/docs#installation")
	}

	dir, err := unpack()
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}

	apply := exec.CommandContext(ctx, atlas, "migrate", "apply", "--dir", "file://"+dir, "--url", databaseURL)
	apply.Stdout = os.Stdout
	apply.Stderr = os.Stderr
	if err := apply.Run(); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
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
