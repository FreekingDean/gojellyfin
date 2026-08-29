package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func resetPasswordCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "resetpassword <username>",
		Short: "Reset a user's password, reading the new one from stdin",
		Long: "Resets a password for a user who cannot log in, which the API cannot do:\n" +
			"ForgotPassword answers ContactAdmin because the server has no channel that\n" +
			"reaches the account holder and nobody else.\n\n" +
			"The password is read from stdin, which keeps it out of the shell history\n" +
			"and the process list. Every session the account already had is revoked.",
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
				service := users.New(client)
				user, err := service.UserByUsername(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if err := service.SetPassword(cmd.Context(), user.ID, hash); err != nil {
					return err
				}
				if err := sessions.New(client, activity.New(client)).RevokeForUser(cmd.Context(), user.ID); err != nil {
					return err
				}

				fmt.Printf("reset %s (%s)\n", user.Name, user.ID)

				return nil
			})
		},
	}
}
