package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var userItemData = &gormigrate.Migration{
	ID: "0008_user_item_data",
	Migrate: func(tx *gorm.DB) error {
		type UserItemDatum struct {
			ID                    uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
			UserID                uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_user_item_data_user_item"`
			ItemID                uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_user_item_data_user_item;index"`
			Played                bool
			PlayCount             int32
			IsFavorite            bool
			PlaybackPositionTicks int64
			LastPlayedDate        *time.Time
			CreatedAt             time.Time
			UpdatedAt             time.Time
		}

		return tx.AutoMigrate(&UserItemDatum{})
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable("user_item_data")
	},
}
