package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var createUsers = &gormigrate.Migration{
	ID: "0001_create_users",
	Migrate: func(tx *gorm.DB) error {
		type User struct {
			ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
			Name      string
			Username  string `gorm:"uniqueIndex"`
			CreatedAt time.Time
			UpdatedAt time.Time
		}

		return tx.AutoMigrate(&User{})
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable("users")
	},
}
