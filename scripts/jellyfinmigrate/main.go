package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/scripts/jellyfinmigrate/migration"
)

func main() {
	var (
		from   string
		dryRun bool
	)

	command := &cobra.Command{
		Use:   "jellyfinmigrate",
		Short: "Carry a Jellyfin install's users, sign-ins and watch state into gojellyfin",
		Long: "Reads jellyfin.db, library.db and device.txt out of the directory Jellyfin keeps\n" +
			"them in, read only, and writes the users, their signed-in devices, the api keys\n" +
			"and the watch state into the database DATABASE_URL names.\n\n" +
			"Everything is one transaction, and --dry-run is the same run rolled back.\n" +
			"Running it twice carries nothing twice.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), from, dryRun)
		},
	}
	command.Flags().StringVar(&from, "from", "", "the directory holding Jellyfin's jellyfin.db, such as /config/data")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be carried without writing anything")
	_ = command.MarkFlagRequired("from")

	if err := command.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, from string, dryRun bool) error {
	source, err := migration.Read(ctx, from)
	if err != nil {
		return err
	}

	config, err := env.Load()
	if err != nil {
		return err
	}
	db, err := store.NewStore(config)
	if err != nil {
		return err
	}
	if err := db.Start(); err != nil {
		return err
	}
	defer func() { _ = db.Stop() }()

	report, err := migration.Run(ctx, db.Client(), source, dryRun)
	if err != nil {
		return err
	}
	fmt.Print(report)

	return nil
}
