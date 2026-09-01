package items

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type (
	Image       = store.Image
	ImageKind   = imagemodal.Kind
	ImageSource = imagemodal.Source
)

const (
	ImageKindPrimary = imagemodal.KindPrimary

	ImageSourceLocal  = imagemodal.SourceLocal
	ImageSourceRemote = imagemodal.SourceRemote
)

var ValidImageKind = imagemodal.KindValidator

type Artwork struct {
	Kind   ImageKind
	Path   string
	Tag    string
	Width  int32
	Height int32
	Size   int64
}

type RemoteImage struct {
	Kind ImageKind
	URL  string
}

func (s *Service) SaveImage(ctx context.Context, itemID uuid.UUID, artwork Artwork) error {
	_, err := s.store.Image.Delete().
		Where(
			imagemodal.ItemID(itemID),
			imagemodal.KindEQ(artwork.Kind),
			imagemodal.Index(0),
			imagemodal.SourceEQ(imagemodal.SourceRemote),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to displace downloaded image: %w", err)
	}

	err = s.store.Image.Create().
		SetItemID(itemID).
		SetKind(artwork.Kind).
		SetPath(artwork.Path).
		SetTag(artwork.Tag).
		SetWidth(artwork.Width).
		SetHeight(artwork.Height).
		SetSize(artwork.Size).
		OnConflictColumns(imagemodal.FieldItemID, imagemodal.FieldKind, imagemodal.FieldIndex).
		DoNothing().
		Exec(ctx)
	if err != nil && !errors.Is(err, stdsql.ErrNoRows) {
		return fmt.Errorf("failed to save image: %w", err)
	}

	return nil
}

func (s *Service) SaveDownloadedImage(ctx context.Context, itemID uuid.UUID, artwork Artwork) error {
	replaced, err := s.store.Image.Update().
		Where(
			imagemodal.ItemID(itemID),
			imagemodal.KindEQ(artwork.Kind),
			imagemodal.Index(0),
			imagemodal.SourceEQ(imagemodal.SourceRemote),
		).
		SetPath(artwork.Path).
		SetTag(artwork.Tag).
		SetSize(artwork.Size).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to replace downloaded image: %w", err)
	}
	if replaced > 0 {
		return nil
	}

	err = s.store.Image.Create().
		SetItemID(itemID).
		SetKind(artwork.Kind).
		SetSource(imagemodal.SourceRemote).
		SetPath(artwork.Path).
		SetTag(artwork.Tag).
		SetSize(artwork.Size).
		OnConflictColumns(imagemodal.FieldItemID, imagemodal.FieldKind, imagemodal.FieldIndex).
		DoNothing().
		Exec(ctx)
	if err != nil && !errors.Is(err, stdsql.ErrNoRows) {
		return fmt.Errorf("failed to save downloaded image: %w", err)
	}

	return nil
}

func (s *Service) DeleteImagesNotInPaths(ctx context.Context, libraryID uuid.UUID, paths []string) error {
	missing := s.store.Image.Delete().Where(
		imagemodal.HasItemWith(itemmodal.LibraryID(libraryID)),
		imagemodal.SourceEQ(imagemodal.SourceLocal),
	)
	if len(paths) > 0 {
		missing = missing.Where(imagemodal.PathNotIn(paths...))
	}

	if _, err := missing.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete missing images: %w", err)
	}

	return nil
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
				itemmodal.LibraryID(libraryID),
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
