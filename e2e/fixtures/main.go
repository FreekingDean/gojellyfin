package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/source"
)

const (
	library = "Smoke Movies"
	shows   = "Smoke Shows"
	series  = "Fixture Show"
	season  = "Season 1"
)

var (
	movies   = []string{"Fixture Alpha", "Fixture Beta"}
	episodes = []string{"Fixture Pilot", "Fixture Second"}
)

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

	downloader, err := client.Source.Create().
		SetName("Fixtures").
		SetURL("http://fixtures.invalid").
		SetAPIKeyVariable(env.SourceAPIKeyPrefix + "FIXTURES").
		SetKind(sourcemodal.KindRadarr).
		SetRootPath("/fixtures").
		SetLocalPath("/fixtures").
		Save(ctx)
	if err != nil {
		return err
	}

	catalogue := items.New(client)
	for _, name := range movies {
		item, err := catalogue.SaveScanned(ctx, items.Scanned{
			Kind:         itemmodal.KindMovie,
			Key:          "movie:" + slugify(name),
			Name:         name,
			SortName:     strings.ToLower(name),
			DateModified: time.Now(),
		})
		if err != nil {
			return err
		}

		if err := file(ctx, catalogue, downloader.ID, item.ID, name); err != nil {
			return err
		}
		if err := member(ctx, client, record.ID, downloader.ID, item.ID); err != nil {
			return err
		}
	}

	shown, err := libraries.New(client).CreateLibrary(ctx, shows, libraries.CollectionTypeTvshows, []string{"/fixtures/shows"})
	if err != nil {
		return err
	}

	number := int32(1)

	show, err := catalogue.SaveScanned(ctx, items.Scanned{
		Kind:         itemmodal.KindSeries,
		Key:          "series:" + slugify(series),
		Name:         series,
		SortName:     strings.ToLower(series),
		DateModified: time.Now(),
	})
	if err != nil {
		return err
	}

	first, err := catalogue.SaveScanned(ctx, items.Scanned{
		ParentID:     &show.ID,
		Kind:         itemmodal.KindSeason,
		Key:          "season:" + slugify(series) + ":1",
		Name:         season,
		SortName:     strings.ToLower(season),
		IndexNumber:  &number,
		DateModified: time.Now(),
	})
	if err != nil {
		return err
	}

	for index, name := range episodes {
		position := int32(index + 1)
		item, err := catalogue.SaveScanned(ctx, items.Scanned{
			ParentID:          &first.ID,
			Kind:              itemmodal.KindEpisode,
			Key:               fmt.Sprintf("episode:%s:1:%d", slugify(series), position),
			Name:              name,
			SortName:          strings.ToLower(name),
			IndexNumber:       &position,
			ParentIndexNumber: &number,
			DateModified:      time.Now(),
		})
		if err != nil {
			return err
		}

		if err := file(ctx, catalogue, downloader.ID, item.ID, name); err != nil {
			return err
		}
		if err := member(ctx, client, shown.ID, downloader.ID, item.ID); err != nil {
			return err
		}
	}

	for _, id := range []uuid.UUID{show.ID, first.ID} {
		if err := member(ctx, client, shown.ID, downloader.ID, id); err != nil {
			return err
		}
	}

	fmt.Println(record.ID)

	return nil
}

func member(ctx context.Context, client *store.Client, libraryID, sourceID, itemID uuid.UUID) error {
	return client.LibraryItem.Create().
		SetLibraryID(libraryID).
		SetSourceID(sourceID).
		SetItemID(itemID).
		Exec(ctx)
}

func file(ctx context.Context, catalogue *items.Service, sourceID, itemID uuid.UUID, name string) error {
	_, err := catalogue.SaveSource(ctx, items.ScannedSource{
		SourceID:     sourceID,
		ItemID:       itemID,
		Path:         "/fixtures/" + name + ".mkv",
		Name:         name,
		DateModified: time.Now(),
	})

	return err
}

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
