package config

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"gorm.io/gorm/clause"

	"github.com/FreekingDean/gojellyfin/internal/store"
)

// Server identity, shared by every DTO that carries a ServerId.
const (
	ServerID     = "e10a32fca79342d7b8b9d96e255ce1bc"
	RootFolderID = "e9d5075a555c1cbc394eec4cef295274"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

type Configuration struct {
	Key       string `gorm:"primaryKey"`
	Value     store.JSON
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Service) Configuration(ctx context.Context, key string) (store.JSON, error) {
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

func (s *Service) SetConfiguration(ctx context.Context, key string, value store.JSON) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&Configuration{Key: key, Value: value}).Error
}
