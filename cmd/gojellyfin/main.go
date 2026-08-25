package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

func main() {
	root := &cobra.Command{
		Use:          "gojellyfin",
		Short:        "A Jellyfin media server and the operator tasks that go with it",
		SilenceUsage: true,
	}
	root.AddCommand(
		serverCommand(),
		workerCommand(),
		migrateCommand(),
		addUserCommand(),
		resetPasswordCommand(),
		importCommand(),
		localizationDataCommand(),
	)

	if err := root.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func withStore(f func(*store.Client) error) error {
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

	return f(db.Client())
}

// Piping the password in keeps it out of the shell history and the process
// list, which a flag would not.
func readPassword() (string, error) {
	fmt.Fprint(os.Stderr, "password: ")
	password, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	password = strings.TrimRight(password, "\r\n")
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}

	return password, nil
}
