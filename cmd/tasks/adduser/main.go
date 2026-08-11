// Creates the first user, which the API cannot do: CreateUserByName requires
// an administrator, so a fresh database has no way in.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func main() {
	name := flag.String("name", "", "user name, also used as the username")
	admin := flag.Bool("admin", true, "grant administrator rights")
	flag.Parse()

	if *name == "" {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(context.Background(), *name, *admin); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, name string, admin bool) error {
	password, err := readPassword()
	if err != nil {
		return err
	}

	hash, err := auth.Hash(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	db, err := store.NewStore()
	if err != nil {
		return err
	}
	if err := db.Start(); err != nil {
		return err
	}
	defer func() { _ = db.Stop() }()

	user, err := users.New(db.Client()).CreateUser(ctx, name, hash, admin)
	if err != nil {
		return err
	}

	fmt.Printf("created %s (%s)\n", user.Name, user.ID)

	return nil
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
