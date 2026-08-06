package config

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"gorm.io/gorm/clause"
)

type Configuration struct {
	Key       string `gorm:"primaryKey"`
	Value     store.JSON
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Server) configuration(ctx context.Context, key string) (store.JSON, error) {
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

func (s *Server) setConfiguration(ctx context.Context, key string, value store.JSON) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&Configuration{Key: key, Value: value}).Error
}
