package store

import (
	"context"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/store/migrations"
)

var Module = fx.Module(
	"store",
	fx.Provide(
		NewDB,
		New,
	),
	fx.Invoke(
		migrations.Run,
		run,
	),
)

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
