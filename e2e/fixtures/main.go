package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const library = "Smoke Movies"

var movies = []string{"Fixture Alpha", "Fixture Beta"}

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: fixtures create|drop|seed"))
	}

	var err error
	switch os.Args[1] {
	case "create":
		err = create()
	case "drop":
		err = drop()
	case "seed":
		err = seed()
	default:
		err = fmt.Errorf("unknown action %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func create() error {
	name, err := scratchName()
	if err != nil {
		return err
	}

	if err := admin("CREATE DATABASE " + name); err != nil {
		return err
	}

	fmt.Println(name)

	return nil
}

func drop() error {
	name, err := scratchName()
	if err != nil {
		return err
	}

	return admin("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
}

func scratchName() (string, error) {
	name := os.Getenv("SCRATCH_DATABASE")
	if name == "" {
		return "", fmt.Errorf("SCRATCH_DATABASE is not set")
	}
	if strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" {
		return "", fmt.Errorf("SCRATCH_DATABASE %q is not a bare identifier", name)
	}

	return name, nil
}

// Postgres does not parameterise a database name; scratchName is what keeps
// the interpolation an identifier.
func admin(statement string) error {
	dsn := os.Getenv("ADMIN_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("ADMIN_DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), statement); err != nil {
		return fmt.Errorf("%q failed: %w", statement, err)
	}

	return nil
}

func seed() error {
	config, err := env.Load()
	if err != nil {
		return err
	}

	connection, err := store.NewStore(config)
	if err != nil {
		return err
	}
	if err := connection.Start(); err != nil {
		return err
	}
	defer func() { _ = connection.Stop() }()

	ctx := context.Background()
	client := connection.Client()

	record, err := libraries.New(client).CreateLibrary(ctx, library, libraries.CollectionTypeMovies, []string{"/fixtures"})
	if err != nil {
		return err
	}

	catalogue := items.New(client)
	for _, name := range movies {
		item, err := catalogue.SaveScanned(ctx, items.Scanned{
			LibraryID:    record.ID,
			Kind:         itemmodal.KindMovie,
			Key:          "movie:" + slugify(name),
			Name:         name,
			SortName:     strings.ToLower(name),
			DateModified: time.Now(),
		})
		if err != nil {
			return err
		}

		_, err = catalogue.SaveSource(ctx, items.ScannedSource{
			LibraryID:    record.ID,
			ItemID:       item.ID,
			Path:         "/fixtures/" + name + ".mkv",
			Name:         name,
			DateModified: time.Now(),
		})
		if err != nil {
			return err
		}
	}

	fmt.Println(record.ID)

	return nil
}

// The scanner derives a key from the title and keeps it unexported, so this
// mirrors the shape rather than sharing it. Two fixtures must not slug alike:
// one key is one title, and they would seed as a single item.
func slugify(name string) string {
	var slug strings.Builder
	separated := false
	for _, letter := range strings.ToLower(name) {
		if !unicode.IsLetter(letter) && !unicode.IsDigit(letter) {
			separated = slug.Len() > 0
			continue
		}
		if separated {
			slug.WriteByte('-')
			separated = false
		}
		slug.WriteRune(letter)
	}

	return slug.String()
}
