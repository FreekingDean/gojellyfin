package libraries

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/store"
)

type Library struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
	Name           string    `gorm:"uniqueIndex"`
	CollectionType string
	Options        store.JSON
	Paths          []LibraryPath `gorm:"foreignKey:LibraryID;constraint:OnDelete:CASCADE"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type LibraryPath struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
	LibraryID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_library_paths_library_path"`
	Path      string    `gorm:"uniqueIndex:idx_library_paths_library_path"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Server) CreateLibrary(ctx context.Context, library *Library) error {
	return s.db.WithContext(ctx).Create(library).Error
}

func (s *Server) GetLibrary(ctx context.Context, id uuid.UUID) (*Library, error) {
	var library Library
	if err := s.db.WithContext(ctx).Preload("Paths").First(&library, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &library, nil
}

func (s *Server) GetLibraryByName(ctx context.Context, name string) (*Library, error) {
	var library Library
	if err := s.db.WithContext(ctx).Preload("Paths").First(&library, "name = ?", name).Error; err != nil {
		return nil, err
	}

	return &library, nil
}

func (s *Server) ListLibraries(ctx context.Context) ([]Library, error) {
	var libraries []Library
	if err := s.db.WithContext(ctx).Preload("Paths").Find(&libraries).Error; err != nil {
		return nil, err
	}

	return libraries, nil
}

func (s *Server) UpdateLibrary(ctx context.Context, library *Library) error {
	return s.db.WithContext(ctx).Omit("Paths").Save(library).Error
}

func (s *Server) DeleteLibrary(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&LibraryPath{}, "library_id = ?", id).Error; err != nil {
			return err
		}

		return tx.Delete(&Library{}, "id = ?", id).Error
	})
}

func (s *Server) AddLibraryPath(ctx context.Context, libraryID uuid.UUID, path string) error {
	err := s.db.WithContext(ctx).Create(&LibraryPath{LibraryID: libraryID, Path: path}).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return nil
	}

	return err
}

func (s *Server) RemoveLibraryPath(ctx context.Context, libraryID uuid.UUID, path string) error {
	return s.db.WithContext(ctx).Delete(&LibraryPath{}, "library_id = ? AND path = ?", libraryID, path).Error
}
