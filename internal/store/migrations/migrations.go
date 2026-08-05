package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var all = []*gormigrate.Migration{
	createUsers,
	userAuth,
	seedAdmin,
	bcryptPasswords,
}

func Run(db *gorm.DB) error {
	options := *gormigrate.DefaultOptions
	options.UseTransaction = true

	return gormigrate.New(db, &options, all).Migrate()
}
