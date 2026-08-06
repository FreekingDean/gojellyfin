package items

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ItemImage struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
	ItemID    uuid.UUID `gorm:"type:uuid;index;uniqueIndex:idx_item_images_item_type_index"`
	Type      string    `gorm:"uniqueIndex:idx_item_images_item_type_index"`
	Index     int32     `gorm:"uniqueIndex:idx_item_images_item_type_index"`
	Path      string
	Tag       string
	Width     int32
	Height    int32
	Size      int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Service) ReplaceImages(ctx context.Context, itemID uuid.UUID, images []ItemImage) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&ItemImage{}, "item_id = ?", itemID).Error; err != nil {
			return err
		}
		if len(images) == 0 {
			return nil
		}

		return tx.Create(&images).Error
	})
}

func (s *Service) Images(ctx context.Context, itemID uuid.UUID) ([]ItemImage, error) {
	var images []ItemImage
	err := s.db.WithContext(ctx).
		Where("item_id = ?", itemID).
		Order("type, index").
		Find(&images).Error

	return images, err
}

func (s *Service) Image(ctx context.Context, itemID uuid.UUID, imageType string, index int32) (*ItemImage, error) {
	var image ItemImage
	err := s.db.WithContext(ctx).
		First(&image, "item_id = ? AND type = ? AND index = ?", itemID, imageType, index).Error
	if err != nil {
		return nil, err
	}

	return &image, nil
}

// ImageTags is the {type: tag} map every item DTO carries, for whole result
// sets at once.
func (s *Service) ImageTags(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]map[string]string, error) {
	tags := map[uuid.UUID]map[string]string{}
	if len(itemIDs) == 0 {
		return tags, nil
	}

	var images []ItemImage
	err := s.db.WithContext(ctx).
		Where("item_id IN ? AND index = 0", itemIDs).
		Find(&images).Error
	if err != nil {
		return nil, err
	}

	for _, image := range images {
		if tags[image.ItemID] == nil {
			tags[image.ItemID] = map[string]string{}
		}
		tags[image.ItemID][image.Type] = image.Tag
	}

	return tags, nil
}
