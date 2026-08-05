package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/auth"
)

var bcryptPasswords = &gormigrate.Migration{
	ID: "0004_bcrypt_passwords",
	Migrate: func(tx *gorm.DB) error {
		hash, err := auth.Hash(env("JELLYFIN_ADMIN_PASSWORD", "admin"))
		if err != nil {
			return err
		}

		return tx.Exec(
			`UPDATE users SET password_hash = ? WHERE username = ? AND password_hash NOT LIKE '$2%'`,
			hash, env("JELLYFIN_ADMIN_USER", "admin"),
		).Error
	},
	Rollback: func(tx *gorm.DB) error {
		return nil
	},
}
