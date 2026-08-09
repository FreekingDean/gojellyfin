package store

import (
	"database/sql"
	"os"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultDSN = "postgres://localhost:5432/gojellyfin_development?sslmode=disable"

func New() (*Client, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDSN
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	return NewClient(Driver(entsql.OpenDB(dialect.Postgres, db))), nil
}
