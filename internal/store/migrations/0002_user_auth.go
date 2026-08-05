package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var userAuth = &gormigrate.Migration{
	ID: "0002_user_auth",
	Migrate: func(tx *gorm.DB) error {
		type User struct {
			ID               uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
			Name             string
			Username         string `gorm:"uniqueIndex"`
			PasswordHash     string
			IsAdministrator  bool
			Configuration    string `gorm:"type:jsonb"`
			Policy           string `gorm:"type:jsonb"`
			LastLoginDate    *time.Time
			LastActivityDate *time.Time
			CreatedAt        time.Time
			UpdatedAt        time.Time
		}

		type Session struct {
			ID               uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
			UserID           uuid.UUID `gorm:"type:uuid;index"`
			AccessToken      string    `gorm:"uniqueIndex"`
			DeviceID         string
			DeviceName       string
			Client           string
			AppVersion       string
			LastActivityDate time.Time
			CreatedAt        time.Time
			UpdatedAt        time.Time
		}

		return tx.AutoMigrate(&User{}, &Session{})
	},
	Rollback: func(tx *gorm.DB) error {
		if err := tx.Migrator().DropTable("sessions"); err != nil {
			return err
		}

		return tx.Exec(`ALTER TABLE users
			DROP COLUMN password_hash,
			DROP COLUMN is_administrator,
			DROP COLUMN configuration,
			DROP COLUMN policy,
			DROP COLUMN last_login_date,
			DROP COLUMN last_activity_date`).Error
	},
}
