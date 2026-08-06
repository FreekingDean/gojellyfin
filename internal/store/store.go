package store

import (
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultDSN = "postgres://localhost:5432/gojellyfin_development?sslmode=disable"

func NewDB() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDSN
	}

	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}
