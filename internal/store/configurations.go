package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Configuration struct {
	Key       string `gorm:"primaryKey"`
	Value     JSON
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *storeImpl) GetConfiguration(ctx context.Context, key string) (JSON, error) {
	var configuration Configuration
	err := s.db.WithContext(ctx).First(&configuration, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return configuration.Value, nil
}

func (s *storeImpl) SetConfiguration(ctx context.Context, key string, value JSON) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&Configuration{Key: key, Value: value}).Error
}
