package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var libraries = &gormigrate.Migration{
	ID: "0006_libraries",
	Migrate: func(tx *gorm.DB) error {
		type Library struct {
			ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
			Name           string    `gorm:"uniqueIndex"`
			CollectionType string
			Options        string `gorm:"type:jsonb"`
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

		return tx.AutoMigrate(&Library{}, &LibraryPath{})
	},
	Rollback: func(tx *gorm.DB) error {
		if err := tx.Migrator().DropTable("library_paths"); err != nil {
			return err
		}

		return tx.Migrator().DropTable("libraries")
	},
}
