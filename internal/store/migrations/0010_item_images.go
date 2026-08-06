package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var itemImages = &gormigrate.Migration{
	ID: "0010_item_images",
	Migrate: func(tx *gorm.DB) error {
		type ItemImage struct {
			ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
			ItemID    uuid.UUID `gorm:"type:uuid;index;uniqueIndex:idx_item_images_item_type_index"`
			Type      string    `gorm:"uniqueIndex:idx_item_images_item_type_index"`
			Index     int32     `gorm:"uniqueIndex:idx_item_images_item_type_index"`
			Path      string
			Tag       string
			Width     int32
			Height    int32
			Size      int64
			CreatedAt time.Time
			UpdatedAt time.Time
		}

		return tx.AutoMigrate(&ItemImage{})
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable("item_images")
	},
}
