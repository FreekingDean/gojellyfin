package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func addUserCommand() *cobra.Command {
	var admin bool

	command := &cobra.Command{
		Use:   "adduser <name>",
		Short: "Create a user, reading the password from stdin",
		Long: "Creates the first user, which the API cannot do: CreateUserByName requires\n" +
			"an administrator, so a fresh database has no way in.\n\n" +
			"The password is read from stdin, which keeps it out of the shell history\n" +
			"and the process list.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := readPassword()
			if err != nil {
				return err
			}

			hash, err := auth.Hash(password)
			if err != nil {
				return fmt.Errorf("failed to hash password: %w", err)
			}

			return withStore(func(client *store.Client) error {
				user, err := users.New(client).CreateUser(cmd.Context(), args[0], hash, admin)
				if err != nil {
					return err
				}

				fmt.Printf("created %s (%s)\n", user.Name, user.ID)

				return nil
			})
		},
	}
	command.Flags().BoolVar(&admin, "admin", true, "grant administrator rights")

	return command
}
