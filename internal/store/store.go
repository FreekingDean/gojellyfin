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

	UpsertItem(ctx context.Context, item *Item) error
	GetItem(ctx context.Context, id uuid.UUID) (*Item, error)
	GetItemByPath(ctx context.Context, libraryID uuid.UUID, path string) (*Item, error)
	ListItemsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]Item, error)
	ListItemsByParent(ctx context.Context, parentID uuid.UUID) ([]Item, error)
	QueryItems(ctx context.Context, query ItemQuery) ([]Item, int64, error)
	CountChildren(ctx context.Context, parentIDs []uuid.UUID) (map[uuid.UUID]int32, error)
	DeleteItemsNotInPaths(ctx context.Context, libraryID uuid.UUID, paths []string) error

	SaveItemMedia(ctx context.Context, item *Item) error
	ReplaceMediaStreams(ctx context.Context, itemID uuid.UUID, streams []MediaStream) error
	ListMediaStreams(ctx context.Context, itemID uuid.UUID) ([]MediaStream, error)

	GetUserItemDatum(ctx context.Context, userID, itemID uuid.UUID) (*UserItemDatum, error)
	ListUserItemData(ctx context.Context, userID uuid.UUID, itemIDs []uuid.UUID) (map[uuid.UUID]UserItemDatum, error)
	SaveUserItemDatum(ctx context.Context, datum *UserItemDatum) error
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
