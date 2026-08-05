package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var mediaStreams = &gormigrate.Migration{
	ID: "0009_media_streams",
	Migrate: func(tx *gorm.DB) error {
		type Item struct {
			Container string
			Size      int64
			Bitrate   int32
			ProbedAt  *time.Time
		}

		type MediaStream struct {
			ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
			ItemID      uuid.UUID `gorm:"type:uuid;index;uniqueIndex:idx_media_streams_item_index"`
			Index       int32     `gorm:"uniqueIndex:idx_media_streams_item_index"`
			Type        string
			Codec       string
			Profile     string
			Language    string
			Title       string
			Width       int32
			Height      int32
			Channels    int32
			SampleRate  int32
			Bitrate     int32
			PixelFormat string
			Level       int32
			IsDefault   bool
			IsForced    bool
			CreatedAt   time.Time
			UpdatedAt   time.Time
		}

		return tx.AutoMigrate(&Item{}, &MediaStream{})
	},
	Rollback: func(tx *gorm.DB) error {
		if err := tx.Migrator().DropTable("media_streams"); err != nil {
			return err
		}

		return tx.Exec(`ALTER TABLE items
			DROP COLUMN container,
			DROP COLUMN size,
			DROP COLUMN bitrate,
			DROP COLUMN probed_at`).Error
	},
}
