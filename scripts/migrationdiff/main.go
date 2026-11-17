package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"slices"
	"strings"
	"text/template"

	"ariga.io/atlas/sql/migrate"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"

	"github.com/FreekingDean/gojellyfin/internal/env"
	entmigrate "github.com/FreekingDean/gojellyfin/internal/store/migrate"
)

const (
	directory = "internal/store/migrations"
	header    = "SET LOCAL lock_timeout = '30s';\nSET LOCAL client_connection_check_interval = '10s';\n"
)

func main() {
	var check, hash bool

	command := &cobra.Command{
		Use:   "migrationdiff [name]",
		Short: "Write the migration that takes internal/store/migrations to the ent schema",
		Long: "Replays internal/store/migrations into a scratch database it creates on the server\n" +
			"DATABASE_URL names, diffs the result against internal/store/entities and writes\n" +
			"the difference as a new migration, opening with the lock bounds every migration\n" +
			"carries. A schema that matches writes nothing.\n\n" +
			"--check writes nothing either, and fails when there is a difference to write.\n" +
			"--hash rewrites atlas.sum after a migration was edited by hand, and touches no database.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "schema"
			if len(args) == 1 {
				name = args[0]
			}

			if hash {
				return rehash()
			}

			return run(cmd.Context(), name, check)
		},
	}
	command.Flags().BoolVar(&hash, "hash", false, "rewrite atlas.sum over the migrations as they stand")
	command.Flags().BoolVar(&check, "check", false, "fail rather than write when the schema has drifted from the migrations")

	if err := command.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, name string, check bool) error {
	config, err := env.Load()
	if err != nil {
		return err
	}

	scratch, drop, err := scratchDatabase(ctx, config.DatabaseURL)
	if err != nil {
		return err
	}
	defer drop()

	target := directory
	if check {
		if target, err = copyDirectory(directory); err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(target) }()
	}

	before, err := migrations(target)
	if err != nil {
		return err
	}

	if err := diff(ctx, scratch, target, name); err != nil {
		return err
	}

	after, err := migrations(target)
	if err != nil {
		return err
	}

	written := slices.DeleteFunc(after, func(file string) bool { return slices.Contains(before, file) })
	switch {
	case len(written) == 0:
		fmt.Println("the migrations already match the ent schema")
	case check:
		return fmt.Errorf("internal/store/entities has changes no migration carries: run make generate and commit %s", directory)
	default:
		for _, file := range written {
			fmt.Printf("wrote %s/%s: read it, make it safe to re-run, then run make migration-hash\n", directory, file)
		}
	}

	return nil
}

func rehash() error {
	dir, err := migrate.NewLocalDir(directory)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", directory, err)
	}

	sum, err := dir.Checksum()
	if err != nil {
		return fmt.Errorf("failed to hash %s: %w", directory, err)
	}

	return migrate.WriteSumFile(dir, sum)
}

func diff(ctx context.Context, scratch *sql.DB, target, name string) error {
	dir, err := migrate.NewLocalDir(target)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", target, err)
	}

	formatter, err := migrate.NewTemplateFormatter(
		template.Must(template.New("name").Funcs(template.FuncMap{"now": migrate.NewVersion}).Parse(
			"{{ with .Version }}{{ . }}{{ else }}{{ now }}{{ end }}{{ with .Name }}_{{ . }}{{ end }}.sql",
		)),
		template.Must(template.New("content").Funcs(template.FuncMap{"stamp": stampTrigger}).Parse(
			header+`{{ range .Changes }}{{ printf "%s;\n" .Cmd }}{{ stamp .Cmd }}{{ end }}`,
		)),
	)
	if err != nil {
		return err
	}

	if err := replay(ctx, scratch, dir); err != nil {
		return err
	}

	err = entmigrate.NewSchema(entsql.OpenDB(dialect.Postgres, scratch)).NamedDiff(ctx, name,
		schema.WithDir(dir),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(formatter),
	)
	if err != nil {
		return fmt.Errorf("failed to diff the ent schema against %s: %w", target, err)
	}

	return nil
}

func stampTrigger(cmd string) string {
	rest, created := strings.CutPrefix(cmd, `CREATE TABLE "`)
	if !created || !strings.Contains(rest, `"updated_at"`) {
		return ""
	}
	table, _, _ := strings.Cut(rest, `"`)

	return fmt.Sprintf("CREATE TRIGGER stamp_updated_at BEFORE UPDATE ON %q FOR EACH ROW EXECUTE FUNCTION stamp_updated_at();\n", table)
}

func replay(ctx context.Context, scratch *sql.DB, dir migrate.Dir) error {
	files, err := dir.Files()
	if err != nil {
		return fmt.Errorf("failed to read the migrations: %w", err)
	}

	for _, file := range files {
		tx, err := scratch.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(file.Bytes())); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to replay %s: %w", file.Name(), err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to replay %s: %w", file.Name(), err)
		}
	}

	return nil
}

func scratchDatabase(ctx context.Context, databaseURL string) (*sql.DB, func(), error) {
	server, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to reach the server DATABASE_URL names: %w", err)
	}

	name := "gojellyfin_migrationdiff_" + uuid.New().String()[:8]
	if _, err := server.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		_ = server.Close(ctx)
		return nil, nil, fmt.Errorf("failed to create a scratch database: %w", err)
	}

	scratchURL, err := url.Parse(databaseURL)
	if err != nil {
		_ = server.Close(ctx)
		return nil, nil, fmt.Errorf("failed to read DATABASE_URL: %w", err)
	}
	scratchURL.Path = "/" + name
	query := scratchURL.Query()
	query.Set("search_path", "public")
	scratchURL.RawQuery = query.Encode()

	connection, err := pgx.ParseConfig(scratchURL.String())
	if err != nil {
		_ = server.Close(ctx)
		return nil, nil, err
	}
	scratch := stdlib.OpenDB(*connection)

	drop := func() {
		_ = scratch.Close()
		if _, err := server.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()); err != nil {
			fmt.Fprintf(os.Stderr, "failed to drop the scratch database %s: %v\n", name, err)
		}
		_ = server.Close(ctx)
	}

	return scratch, drop, nil
}

func migrations(dir string) ([]string, error) {
	return fs.Glob(os.DirFS(dir), "*.sql")
}

func copyDirectory(from string) (string, error) {
	to, err := os.MkdirTemp("", "migrationdiff")
	if err != nil {
		return "", err
	}
	if err := os.CopyFS(to, os.DirFS(from)); err != nil {
		_ = os.RemoveAll(to)
		return "", fmt.Errorf("failed to copy %s: %w", from, err)
	}

	return to, nil
}
