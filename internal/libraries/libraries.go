package libraries

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/entities"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
	optionsmodal "github.com/FreekingDean/gojellyfin/internal/store/libraryoptions"
)

type (
	Library           = store.Library
	Options           = store.LibraryOptions
	CollectionType    = librarymodal.CollectionType
	EmbeddedSubtitles = optionsmodal.AllowEmbeddedSubtitles
	TypeOptions       = entities.TypeOptions
)

const (
	CollectionTypeMovies      = librarymodal.CollectionTypeMovies
	CollectionTypeTvshows     = librarymodal.CollectionTypeTvshows
	CollectionTypeMusic       = librarymodal.CollectionTypeMusic
	CollectionTypeMusicvideos = librarymodal.CollectionTypeMusicvideos
	CollectionTypeHomevideos  = librarymodal.CollectionTypeHomevideos
	CollectionTypeBoxsets     = librarymodal.CollectionTypeBoxsets
	CollectionTypeBooks       = librarymodal.CollectionTypeBooks
	CollectionTypeMixed       = librarymodal.CollectionTypeMixed
)

var (
	ValidCollectionType = librarymodal.CollectionTypeValidator

	CollectionTypes = []CollectionType{
		CollectionTypeMovies,
		CollectionTypeTvshows,
		CollectionTypeMusic,
		CollectionTypeMusicvideos,
		CollectionTypeHomevideos,
		CollectionTypeBoxsets,
		CollectionTypeBooks,
		CollectionTypeMixed,
	}
)

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) CreateLibrary(ctx context.Context, name string, collectionType CollectionType, locations []string) (*Library, error) {
	var id uuid.UUID
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		library, err := tx.Library.Create().
			SetName(name).
			SetCollectionType(collectionType).
			SetLocations(locations).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create library: %w", err)
		}
		id = library.ID

		if _, err := tx.LibraryOptions.Create().SetLibrary(library).Save(ctx); err != nil {
			return fmt.Errorf("failed to create library options: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.Library(ctx, id)
}

func (s *Service) Library(ctx context.Context, id uuid.UUID) (*Library, error) {
	library, err := s.store.Library.Query().
		Where(librarymodal.ID(id)).
		WithOptions().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query library: %w", err)
	}

	return library, nil
}

func (s *Service) LibraryByName(ctx context.Context, name string) (*Library, error) {
	library, err := s.store.Library.Query().
		Where(librarymodal.Name(name)).
		WithOptions().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query library by name: %w", err)
	}

	return library, nil
}

func (s *Service) ListLibraries(ctx context.Context) ([]*Library, error) {
	libraries, err := s.store.Library.Query().
		WithOptions().
		Order(librarymodal.ByName()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list libraries: %w", err)
	}

	return libraries, nil
}

func (s *Service) Rename(ctx context.Context, id uuid.UUID, name string) error {
	if err := s.store.Library.UpdateOneID(id).SetName(name).Exec(ctx); err != nil {
		return fmt.Errorf("failed to rename library: %w", err)
	}

	return nil
}

func (s *Service) DeleteLibrary(ctx context.Context, id uuid.UUID) error {
	if err := s.store.Library.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete library: %w", err)
	}

	return nil
}

// The caller chains the fields it wants changed; every field has a
// SetNillable form, which is the shape the api sends them in.
func (s *Service) UpdateOptions(id uuid.UUID) *store.LibraryOptionsUpdate {
	return s.store.LibraryOptions.Update().
		Where(optionsmodal.HasLibraryWith(librarymodal.ID(id)))
}

func (s *Service) AddLocation(ctx context.Context, id uuid.UUID, path string) error {
	library, err := s.Library(ctx, id)
	if err != nil {
		return err
	}
	if slices.Contains(library.Locations, path) {
		return nil
	}

	err = s.store.Library.UpdateOneID(id).
		SetLocations(append(library.Locations, path)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to add library location: %w", err)
	}

	return nil
}

func (s *Service) RemoveLocation(ctx context.Context, id uuid.UUID, path string) error {
	library, err := s.Library(ctx, id)
	if err != nil {
		return err
	}

	locations := slices.DeleteFunc(slices.Clone(library.Locations), func(location string) bool {
		return location == path
	})

	if err := s.store.Library.UpdateOneID(id).SetLocations(locations).Exec(ctx); err != nil {
		return fmt.Errorf("failed to remove library location: %w", err)
	}

	return nil
}
