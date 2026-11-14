package fixture

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Layout string

const (
	Legacy       Layout = "legacy"
	Consolidated Layout = "consolidated"
)

const (
	ServerID = "5f0a1b2c3d4e4f6a8b9c0d1e2f3a4b5c"

	AliceID        = "11111111-1111-4111-8111-111111111111"
	AliceUsername  = "alice"
	AlicePassword  = "alice-password"
	AliceWebToken  = "a11ce0000000000000000000000000a1"
	AliceWebDevice = "device-alice-web"
	AliceTVToken   = "a11ce0000000000000000000000000a2"
	AliceOldToken  = "a11ce0000000000000000000000000a9"

	BobID    = "22222222-2222-4222-8222-222222222222"
	BobToken = "b0b00000000000000000000000000b01"

	CarolID       = "33333333-3333-4333-8333-333333333333"
	CarolUsername = "Carol"
	CarolToken    = "ca401000000000000000000000000c01"

	APIKey = "ap1ke70000000000000000000000f001"

	MovieTmdb       = 990001
	ImdbOnlyTmdb    = 990002
	ImdbOnlyImdb    = "tt9900002"
	PathOnlyPath    = "/media/movies/Path Only (2001)/Path Only (2001).mkv"
	SeriesTmdb      = 990010
	ImdbSeriesTmdb  = 990020
	ImdbSeriesImdb  = "tt9900020"
	UnmatchedTitle  = "Gone Movie"
	ResumePosition  = 1234567890
	EpisodePosition = 9000000000
)

//go:embed legacy/*.sql consolidated/*.sql
var schemas embed.FS

func Write(directory string, layout Layout) error {
	entries, err := schemas.ReadDir(string(layout))
	if err != nil {
		return fmt.Errorf("no fixture for the %s layout: %w", layout, err)
	}

	for _, entry := range entries {
		script, err := schemas.ReadFile(path.Join(string(layout), entry.Name()))
		if err != nil {
			return err
		}

		name := entry.Name()[:len(entry.Name())-len(".sql")] + ".db"
		if err := load(filepath.Join(directory, name), string(script)); err != nil {
			return err
		}
	}

	return os.WriteFile(filepath.Join(directory, "device.txt"), []byte(ServerID), 0o600)
}

func load(file, script string) error {
	db, err := sql.Open("sqlite", "file:"+file)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", file, err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(script); err != nil {
		return fmt.Errorf("failed to fill %q: %w", file, err)
	}

	return nil
}
