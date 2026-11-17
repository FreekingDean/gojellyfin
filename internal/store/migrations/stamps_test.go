package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func migrated(t *testing.T) *pgx.Conn {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	conn, err := pgx.Connect(context.Background(), config.DatabaseURL)
	if err != nil {
		t.Fatalf("failed to reach the database: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	return conn
}

func TestEveryUpdatedAtIsStampedByTheDatabase(t *testing.T) {
	conn := migrated(t)

	rows, err := conn.Query(context.Background(), `
		SELECT c.table_name
		FROM information_schema.columns c
		WHERE c.table_schema = current_schema()
		  AND c.column_name = 'updated_at'
		  AND NOT EXISTS (
		    SELECT 1 FROM information_schema.triggers t
		    WHERE t.event_object_schema = c.table_schema
		      AND t.event_object_table = c.table_name
		      AND t.trigger_name = 'stamp_updated_at'
		      AND t.event_manipulation = 'UPDATE'
		  )`)
	if err != nil {
		t.Fatal(err)
	}
	unstamped, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}

	for _, table := range unstamped {
		t.Errorf("%s has an updated_at no trigger stamps: give it stamp_updated_at in the migration that creates it", table)
	}
}

func TestARowWrittenOutsideEntIsStamped(t *testing.T) {
	conn := migrated(t)
	ctx := context.Background()

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	var id string
	var created, inserted time.Time
	if err := tx.QueryRow(ctx,
		`INSERT INTO genres (name) VALUES ('Invented Genre') RETURNING id, created_at, updated_at`,
	).Scan(&id, &created, &inserted); err != nil {
		t.Fatalf("an insert naming no timestamps failed: %v", err)
	}
	if created.IsZero() || inserted.IsZero() {
		t.Fatalf("created_at = %v, updated_at = %v, want both stamped", created, inserted)
	}

	if _, err := tx.Exec(ctx, `UPDATE genres SET updated_at = '2000-01-01', name = 'Renamed Genre' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	var updated time.Time
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM genres WHERE id = $1`, id).Scan(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Year() == 2000 {
		t.Errorf("updated_at = %v: an update chose its own time rather than the database's", updated)
	}
}
