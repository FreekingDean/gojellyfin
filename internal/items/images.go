package items

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type (
	Image     = store.Image
	ImageKind = imagemodal.Kind
)

const (
	ImageKindPrimary = imagemodal.KindPrimary
)

var ValidImageKind = imagemodal.KindValidator

type RemoteImage struct {
	Kind ImageKind
	URL  string
}

func ImageKey(itemID uuid.UUID, kind ImageKind, index int32) string {
	return fmt.Sprintf("artwork/%s/%s/%d", itemID, kind, index)
}

func (s *Service) SaveImage(ctx context.Context, itemID uuid.UUID, artwork Image) error {
	err := s.store.Image.Create().
		SetItemID(itemID).
		SetKind(artwork.Kind).
		SetURL(artwork.URL).
		SetTag(artwork.Tag).
		OnConflictColumns(imagemodal.FieldItemID, imagemodal.FieldKind, imagemodal.FieldIndex).
		UpdateURL().
		UpdateTag().
		UpdateUpdatedAt().
		Update(keyOfTheStoredURL).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save the image: %w", err)
	}

	return nil
}

func keyOfTheStoredURL(upsert *store.ImageUpsert) {
	kept := upsert.Table()
	excluded := sql.Dialect(upsert.Dialect()).Table("excluded")

	upsert.Set(imagemodal.FieldKey, sql.Expr(fmt.Sprintf(
		"CASE WHEN %s = %s THEN %s ELSE '' END",
		kept.C(imagemodal.FieldURL), excluded.C(imagemodal.FieldURL), kept.C(imagemodal.FieldKey),
	)))
}

func (s *Service) ImagesNeedingCache(ctx context.Context, scope uuid.UUID, limit int) ([]*Image, error) {
	query := s.store.Image.Query().Where(imagemodal.Key(""))
	if scope != uuid.Nil {
		query = query.Where(imagemodal.HasItemWith(inLibrary(scope)))
	}

	images, err := query.
		Order(imagemodal.ByCreatedAt(), imagemodal.ByID()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query images needing cache: %w", err)
	}

	return images, nil
}

func (s *Service) SaveImageKey(ctx context.Context, id uuid.UUID, key string) error {
	if err := s.store.Image.UpdateOneID(id).SetKey(key).Exec(ctx); err != nil {
		return fmt.Errorf("failed to save the image key: %w", err)
	}

	return nil
}

func (s *Service) ImageKeys(ctx context.Context, itemIDs []uuid.UUID) ([]string, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	keys, err := s.store.Image.Query().
		Where(imagemodal.ItemIDIn(itemIDs...), imagemodal.KeyNEQ("")).
		Select(imagemodal.FieldKey).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query image keys: %w", err)
	}

	return keys, nil
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

func (s *Service) LibraryPosters(ctx context.Context, libraryID uuid.UUID, limit int) ([]*Image, error) {
	posters, err := s.store.Image.Query().
		Where(
			imagemodal.KindEQ(imagemodal.KindPrimary),
			imagemodal.Index(0),
			imagemodal.HasItemWith(
				inLibrary(libraryID),
				itemmodal.DeletedAtIsNil(),
				itemmodal.ParentIDIsNil(),
			),
		).
		Order(imagemodal.ByCreatedAt(sql.OrderDesc()), imagemodal.ByID()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query library posters: %w", err)
	}

	return posters, nil
}
