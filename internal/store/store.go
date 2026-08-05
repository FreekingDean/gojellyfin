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
	TouchUserLogin(ctx context.Context, id uuid.UUID) error
	TouchUserActivity(ctx context.Context, id uuid.UUID) error

	CreateSession(ctx context.Context, sess *Session) error
	GetSessionByToken(ctx context.Context, token string) (*Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	UpdateSession(ctx context.Context, sess *Session) error
	DeleteSessionByToken(ctx context.Context, token string) error

	GetConfiguration(ctx context.Context, key string) (JSON, error)
	SetConfiguration(ctx context.Context, key string, value JSON) error

	CreateLibrary(ctx context.Context, library *Library) error
	GetLibrary(ctx context.Context, id uuid.UUID) (*Library, error)
	GetLibraryByName(ctx context.Context, name string) (*Library, error)
	ListLibraries(ctx context.Context) ([]Library, error)
	UpdateLibrary(ctx context.Context, library *Library) error
	DeleteLibrary(ctx context.Context, id uuid.UUID) error
	AddLibraryPath(ctx context.Context, libraryID uuid.UUID, path string) error
	RemoveLibraryPath(ctx context.Context, libraryID uuid.UUID, path string) error
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
