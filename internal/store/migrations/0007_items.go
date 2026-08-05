package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var items = &gormigrate.Migration{
	ID: "0007_items",
	Migrate: func(tx *gorm.DB) error {
		type Item struct {
			ID                uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid()"`
			LibraryID         uuid.UUID  `gorm:"type:uuid;index;uniqueIndex:idx_items_library_path"`
			ParentID          *uuid.UUID `gorm:"type:uuid;index"`
			Type              string     `gorm:"index"`
			Name              string
			SortName          string `gorm:"index"`
			Path              string `gorm:"uniqueIndex:idx_items_library_path"`
			Overview          string
			ProductionYear    *int32
			IndexNumber       *int32
			ParentIndexNumber *int32
			PremiereDate      *time.Time
			RunTimeTicks      *int64
			DateModified      time.Time
			CreatedAt         time.Time
			UpdatedAt         time.Time
		}

		return tx.AutoMigrate(&Item{})
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable("items")
	},
}
