package migrations

import (
	"os"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/FreekingDean/gojellyfin/internal/auth"
)

var seedAdmin = &gormigrate.Migration{
	ID: "0003_seed_admin",
	Migrate: func(tx *gorm.DB) error {
		type User struct {
			ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
			Name            string
			Username        string
			PasswordHash    string
			IsAdministrator bool
			CreatedAt       time.Time
			UpdatedAt       time.Time
		}

		hash, err := auth.Hash(env("JELLYFIN_ADMIN_PASSWORD", "admin"))
		if err != nil {
			return err
		}

		username := env("JELLYFIN_ADMIN_USER", "admin")

		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "username"}},
			DoNothing: true,
		}).Create(&User{
			Name:            username,
			Username:        username,
			PasswordHash:    hash,
			IsAdministrator: true,
		}).Error
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.Exec("DELETE FROM users WHERE username = ?", env("JELLYFIN_ADMIN_USER", "admin")).Error
	},
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
