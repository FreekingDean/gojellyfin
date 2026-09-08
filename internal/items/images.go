package items

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type (
	Image     = store.Image
	ImageKind = imagemodel.Kind
)

const (
	ImageKindPrimary = imagemodel.KindPrimary
)

var ValidImageKind = imagemodel.KindValidator

type RemoteImage struct {
	Kind ImageKind
	URL  string
}

func (s *Service) SaveImage(ctx context.Context, itemID uuid.UUID, artwork Image) error {
	err := s.store.Image.Create().
		SetItemID(itemID).
		SetKind(artwork.Kind).
		SetURL(artwork.URL).
		SetTag(artwork.Tag).
		OnConflictColumns(imagemodel.FieldItemID, imagemodal.FieldKind, imagemodal.FieldIndex).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save the image: %w", err)
	}

	return nil
}

func (s *Service) Images(ctx context.Context, itemID uuid.UUID) ([]*Image, error) {
	images, err := s.store.Image.Query().
		Where(imagemodel.ItemID(itemID)).
		Order(imagemodel.ByKind(), imagemodal.ByIndex()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	return images, nil
}

func (s *Service) Image(ctx context.Context, itemID uuid.UUID, kind ImageKind, index int32) (*Image, error) {
	image, err := s.store.Image.Query().
		Where(
			imagemodel.ItemID(itemID),
			imagemodel.KindEQ(kind),
			imagemodel.Index(index),
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
		Where(imagemodel.ItemIDIn(itemIDs...), imagemodal.Index(0)).
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

func (s *Service) LibraryPosters(ctx context.Context, libraryID uuid.UUID, limit int) ([]*Image, error) {
	posters, err := s.store.Image.Query().
		Where(
			imagemodel.KindEQ(imagemodal.KindPrimary),
			imagemodel.Index(0),
			imagemodel.HasItemWith(
				inLibrary(libraryID),
				itemmodel.DeletedAtIsNil(),
				itemmodel.ParentIDIsNil(),
			),
		).
		Order(imagemodel.ByCreatedAt(sql.OrderDesc()), imagemodal.ByID()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query library posters: %w", err)
	}

	return posters, nil
}
