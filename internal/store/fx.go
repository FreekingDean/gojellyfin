package store

import (
	"context"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

var Module = fx.Module(
	"store",
	fx.Provide(
		NewDB,
		New,
	),
	fx.Invoke(
		migrate,
		run,
	),
)

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{})
}

func run(lc fx.Lifecycle, db *gorm.DB) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}

			return sqlDB.Close()
		},
	})
}
