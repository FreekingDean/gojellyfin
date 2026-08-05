package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MediaStream struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
	ItemID      uuid.UUID `gorm:"type:uuid;index;uniqueIndex:idx_media_streams_item_index"`
	Index       int32     `gorm:"uniqueIndex:idx_media_streams_item_index"`
	Type        string
	Codec       string
	Profile     string
	Language    string
	Title       string
	Width       int32
	Height      int32
	Channels    int32
	SampleRate  int32
	Bitrate     int32
	PixelFormat string
	Level       int32
	IsDefault   bool
	IsForced    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *storeImpl) ReplaceMediaStreams(ctx context.Context, itemID uuid.UUID, streams []MediaStream) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&MediaStream{}, "item_id = ?", itemID).Error; err != nil {
			return err
		}
		if len(streams) == 0 {
			return nil
		}

		return tx.Create(&streams).Error
	})
}

func (s *storeImpl) ListMediaStreams(ctx context.Context, itemID uuid.UUID) ([]MediaStream, error) {
	var streams []MediaStream
	if err := s.db.WithContext(ctx).Where("item_id = ?", itemID).Order("index").Find(&streams).Error; err != nil {
		return nil, err
	}

	return streams, nil
}
