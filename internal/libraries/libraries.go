package libraries

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/entities"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"
	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
	optionsmodel "github.com/FreekingDean/gojellyfin/internal/store/libraryoptions"
)

type (
	Library           = store.LibraryModel
	Options           = store.LibraryOptionsModel
	CollectionType    = librarymodel.CollectionType
	EmbeddedSubtitles = optionsmodel.AllowEmbeddedSubtitles
	TypeOptions       = entities.TypeOptions
)

const (
	CollectionTypeMovies      = librarymodel.CollectionTypeMovies
	CollectionTypeTvshows     = librarymodel.CollectionTypeTvshows
	CollectionTypeMusic       = librarymodel.CollectionTypeMusic
	CollectionTypeMusicvideos = librarymodel.CollectionTypeMusicvideos
	CollectionTypeHomevideos  = librarymodel.CollectionTypeHomevideos
	CollectionTypeBoxsets     = librarymodel.CollectionTypeBoxsets
	CollectionTypeBooks       = librarymodel.CollectionTypeBooks
	CollectionTypeMixed       = librarymodel.CollectionTypeMixed
)

var (
	ValidCollectionType = librarymodel.CollectionTypeValidator

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

type Fixed struct {
	ID             uuid.UUID
	Name           string
	CollectionType CollectionType
}

var (
	Movies = Fixed{ID: uuid.MustParse("4d6f7669-6573-4000-8000-000000000001"), Name: "Movies", CollectionType: CollectionTypeMovies}
	Shows  = Fixed{ID: uuid.MustParse("53686f77-7300-4000-8000-000000000002"), Name: "Shows", CollectionType: CollectionTypeTvshows}
	Music  = Fixed{ID: uuid.MustParse("4d757369-6300-4000-8000-000000000003"), Name: "Music", CollectionType: CollectionTypeMusic}

	ErrFixed = errors.New("there are three libraries, Movies, Shows and Music, and none is added or removed")
)

type Service struct {
	store   *store.Client
	sources *sources.Service
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) UseSources(bindings *sources.Service) {
	s.sources = bindings
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

func (s *Service) EnsureFixed(ctx context.Context) error {
	for _, fixed := range []Fixed{Movies, Shows, Music} {
		err := s.ensure(ctx, fixed)
		if store.IsConstraintError(err) {
			err = s.ensure(ctx, fixed)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) ensure(ctx context.Context, fixed Fixed) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		library, err := tx.Library.Query().
			Where(librarymodel.CollectionTypeEQ(fixed.CollectionType)).
			Order(librarymodel.ByCreatedAt(), librarymodel.ByID()).
			WithOptions().
			First(ctx)
		if store.IsNotFound(err) {
			library, err = tx.Library.Create().
				SetID(fixed.ID).
				SetName(fixed.Name).
				SetCollectionType(fixed.CollectionType).
				Save(ctx)
		}
		if err != nil {
			return fmt.Errorf("failed to find or create %s: %w", fixed.Name, err)
		}
		if library.Edges.Options != nil {
			return nil
		}

		if err := tx.LibraryOptions.Create().SetLibrary(library).Exec(ctx); err != nil {
			return fmt.Errorf("failed to create the options of %s: %w", fixed.Name, err)
		}

		return nil
	})
}

func (s *Service) Library(ctx context.Context, id uuid.UUID) (*Library, error) {
	library, err := s.store.Library.Query().
		Where(librarymodel.ID(id)).
		WithOptions().
		OnlyModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query library: %w", err)
	}

	return library, nil
}

func (s *Service) LibraryByName(ctx context.Context, name string) (*Library, error) {
	library, err := s.store.Library.Query().
		Where(librarymodel.Name(name)).
		WithOptions().
		OnlyModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query library by name: %w", err)
	}

	return library, nil
}

func (s *Service) ListLibraries(ctx context.Context) ([]*Library, error) {
	libraries, err := s.store.Library.Query().
		WithOptions().
		Order(librarymodel.ByName()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list libraries: %w", err)
	}

	return libraries, nil
}

func (s *Service) ShownLibraries(ctx context.Context) ([]*Library, error) {
	libraries, err := s.store.Library.Query().
		Where(librarymodel.Or(librarymodel.CollectionTypeNEQ(CollectionTypeMusic), librarymodel.HasSources())).
		WithOptions().
		Order(librarymodel.ByName()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list the libraries shown: %w", err)
	}

	return libraries, nil
}

var groupableCollectionTypes = []CollectionType{
	CollectionTypeMovies,
	CollectionTypeTvshows,
	CollectionTypeMixed,
}

func (s *Service) GroupableLibraries(ctx context.Context) ([]*Library, error) {
	libraries, err := s.store.Library.Query().
		Where(librarymodel.CollectionTypeIn(groupableCollectionTypes...)).
		WithOptions().
		Order(librarymodel.ByName()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list groupable libraries: %w", err)
	}

	return libraries, nil
}

func (s *Service) PhysicalPaths(ctx context.Context) ([]string, error) {
	libraries, err := s.store.Library.Query().
		Select(librarymodel.FieldLocations).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query library paths: %w", err)
	}

	paths := []string{}
	for _, library := range libraries {
		paths = append(paths, library.Locations...)
	}
	slices.Sort(paths)

	return slices.Compact(paths), nil
}

func (s *Service) Rename(ctx context.Context, id uuid.UUID, name string) error {
	if err := s.store.Library.UpdateOneID(id).SetName(name).Exec(ctx); err != nil {
		return fmt.Errorf("failed to rename library: %w", err)
	}

	return nil
}

func (s *Service) DeleteLibrary(ctx context.Context, id uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		orphaned, err := tx.Item.Query().
			Where(itemmodel.HasLibrariesWith(librarymembership.LibraryID(id))).
			IDs(ctx)
		if err != nil {
			return fmt.Errorf("failed to query the library's items: %w", err)
		}

		if err := tx.Library.DeleteOneID(id).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete library: %w", err)
		}

		if _, err := tx.Item.Delete().
			Where(
				itemmodel.IDIn(orphaned...),
				itemmodel.Not(itemmodel.HasLibraries()),
				itemmodel.Not(itemmodel.HasPlaylist()),
			).
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete the items no library holds: %w", err)
		}

		return nil
	})
}

func (s *Service) UpdateOptions(id uuid.UUID) *store.LibraryOptionsUpdate {
	return s.store.LibraryOptions.Update().
		Where(optionsmodel.HasLibraryWith(librarymodel.ID(id)))
}
