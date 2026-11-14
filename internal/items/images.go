package items

import (
	"context"
	"fmt"
	"slices"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type (
	Image     = store.ImageModel
	ImageKind = imagemodel.Kind
)

const (
	ImageKindPrimary  = imagemodel.KindPrimary
	ImageKindBackdrop = imagemodel.KindBackdrop
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
		OnConflictColumns(imagemodel.FieldItemID, imagemodel.FieldKind, imagemodel.FieldIndex).
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
		Order(imagemodel.ByKind(), imagemodel.ByIndex()).
		AllModels(ctx)
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
		OnlyModel(ctx)
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
		Where(imagemodel.ItemIDIn(itemIDs...), imagemodel.Index(0)).
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
			imagemodel.KindEQ(imagemodel.KindPrimary),
			imagemodel.Index(0),
			imagemodel.HasItemWith(
				inLibrary(libraryID),
				itemmodel.DeletedAtIsNil(),
				itemmodel.ParentIDIsNil(),
			),
		).
		Order(imagemodel.ByCreatedAt(sql.OrderDesc()), imagemodel.ByID()).
		Limit(limit).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query library posters: %w", err)
	}

	return posters, nil
}

func (s *Service) Posters(ctx context.Context, records []*Item, limit int) ([]*Image, error) {
	series, err := s.seriesOf(ctx, records)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		id := record.ID
		if parent, ok := series[record.ID]; ok {
			id = parent.ID
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}

	found, err := s.store.Image.Query().
		Where(
			imagemodel.ItemIDIn(ids...),
			imagemodel.KindEQ(imagemodel.KindPrimary),
			imagemodel.Index(0),
		).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query posters: %w", err)
	}

	byItem := make(map[uuid.UUID]*Image, len(found))
	for _, poster := range found {
		byItem[poster.ItemID] = poster
	}

	posters := make([]*Image, 0, limit)
	for _, id := range ids {
		if poster, ok := byItem[id]; ok && len(posters) < limit {
			posters = append(posters, poster)
		}
	}

	return posters, nil
}
