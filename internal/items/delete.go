package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	datamodel "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
)

func (s *Service) DeleteItem(ctx context.Context, id uuid.UUID) error {
	ids, err := s.subtree(ctx, id)
	if err != nil {
		return err
	}

	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		sourceIDs, err := tx.ItemSource.Query().Where(sourcemodel.ItemIDIn(ids...)).IDs(ctx)
		if err != nil {
			return fmt.Errorf("failed to query media sources: %w", err)
		}
		if len(sourceIDs) > 0 {
			if _, err := tx.MediaStream.Delete().Where(streammodel.ItemSourceIDIn(sourceIDs...)).Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete media streams: %w", err)
			}
			if _, err := tx.ItemSource.Delete().Where(sourcemodel.IDIn(sourceIDs...)).Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete media sources: %w", err)
			}
		}
		if _, err := tx.Image.Delete().Where(imagemodel.ItemIDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete images: %w", err)
		}
		if _, err := tx.UserItemData.Delete().Where(datamodel.ItemIDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user item data: %w", err)
		}
		if _, err := tx.Item.Delete().Where(itemmodel.IDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete items: %w", err)
		}

		return nil
	})
}

func (s *Service) subtree(ctx context.Context, root uuid.UUID) ([]uuid.UUID, error) {
	ids := []uuid.UUID{root}
	for frontier := ids; len(frontier) > 0; {
		children, err := s.query(Everyone).
			Where(itemmodel.ParentIDIn(frontier...), itemmodel.IDNotIn(ids...)).
			IDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query child items: %w", err)
		}
		ids = append(ids, children...)
		frontier = children
	}

	return ids, nil
}

type Remover interface {
	RemoveAll(ctx context.Context, path string) error
}

func (s *Service) DeleteWithFiles(ctx context.Context, ids []uuid.UUID, files Remover) error {
	for _, id := range ids {
		paths, err := s.SourcePaths(ctx, id)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err := files.RemoveAll(ctx, path); err != nil {
				return err
			}
		}
		if err := s.DeleteItem(ctx, id); err != nil {
			return err
		}
	}

	return nil
}
