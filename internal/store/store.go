package store

import (
	"context"
	"os"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultDSN = "postgres://localhost:5432/gojellyfin_development?sslmode=disable"

type Store interface {
	CreateUser(ctx context.Context, u *User) error
	GetUser(ctx context.Context, id uuid.UUID) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdateUser(ctx context.Context, u *User) error
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

type storeImpl struct {
	db *gorm.DB
}

func NewDB() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDSN
	}

	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func New(db *gorm.DB) Store {
	return &storeImpl{db: db}
}
