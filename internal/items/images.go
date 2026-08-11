package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
)

type (
	Image     = store.Image
	ImageKind = imagemodal.Kind
)

var ValidImageKind = imagemodal.KindValidator

// One artwork file found beside the media.
type Artwork struct {
	Kind   ImageKind
	Path   string
	Tag    string
	Width  int32
	Height int32
	Size   int64
}

func (s *Service) ReplaceImages(ctx context.Context, itemID uuid.UUID, artwork []Artwork) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		_, err := tx.Image.Delete().Where(imagemodal.ItemID(itemID)).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to clear images: %w", err)
		}
		if len(artwork) == 0 {
			return nil
		}

		builders := make([]*store.ImageCreate, 0, len(artwork))
		for _, image := range artwork {
			builders = append(builders, tx.Image.Create().
				SetItemID(itemID).
				SetKind(image.Kind).
				SetPath(image.Path).
				SetTag(image.Tag).
				SetWidth(image.Width).
				SetHeight(image.Height).
				SetSize(image.Size))
		}
		if err := tx.Image.CreateBulk(builders...).Exec(ctx); err != nil {
			return fmt.Errorf("failed to create images: %w", err)
		}

		return nil
	})
}

func (s *Service) Images(ctx context.Context, itemID uuid.UUID) ([]*Image, error) {
	images, err := s.store.Image.Query().
		Where(imagemodal.ItemID(itemID)).
		Order(imagemodal.ByKind(), imagemodal.ByIndex()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	return images, nil
}

func (s *Service) Image(ctx context.Context, itemID uuid.UUID, kind ImageKind, index int32) (*Image, error) {
	image, err := s.store.Image.Query().
		Where(
			imagemodal.ItemID(itemID),
			imagemodal.KindEQ(kind),
			imagemodal.Index(index),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query image: %w", err)
	}

	return image, nil
}

func (s *Service) ImageTagsByItem(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]map[string]string, error) {
	tags := map[uuid.UUID]map[string]string{}
	if len(itemIDs) == 0 {
		return tags, nil
	}

	images, err := s.store.Image.Query().
		Where(imagemodal.ItemIDIn(itemIDs...), imagemodal.Index(0)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query image tags: %w", err)
	}

	for _, image := range images {
		if tags[image.ItemID] == nil {
			tags[image.ItemID] = map[string]string{}
		}
		tags[image.ItemID][string(image.Kind)] = image.Tag
	}

	return tags, nil
}
