package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var configurations = &gormigrate.Migration{
	ID: "0005_configurations",
	Migrate: func(tx *gorm.DB) error {
		type Configuration struct {
			Key       string `gorm:"primaryKey"`
			Value     string `gorm:"type:jsonb"`
			CreatedAt time.Time
			UpdatedAt time.Time
		}

		return tx.AutoMigrate(&Configuration{})
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable("configurations")
	},
}
