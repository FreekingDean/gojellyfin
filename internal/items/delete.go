package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	datamodal "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
)

func (s *Service) DeleteItem(ctx context.Context, id uuid.UUID) error {
	ids, err := s.subtree(ctx, id)
	if err != nil {
		return err
	}

	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		sourceIDs, err := tx.MediaSource.Query().Where(sourcemodal.ItemIDIn(ids...)).IDs(ctx)
		if err != nil {
			return fmt.Errorf("failed to query media sources: %w", err)
		}
		if len(sourceIDs) > 0 {
			if _, err := tx.MediaStream.Delete().Where(streammodal.SourceIDIn(sourceIDs...)).Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete media streams: %w", err)
			}
			if _, err := tx.MediaSource.Delete().Where(sourcemodal.IDIn(sourceIDs...)).Exec(ctx); err != nil {
				return fmt.Errorf("failed to delete media sources: %w", err)
			}
		}
		if _, err := tx.Image.Delete().Where(imagemodal.ItemIDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete images: %w", err)
		}
		if _, err := tx.UserItemData.Delete().Where(datamodal.ItemIDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user item data: %w", err)
		}
		if _, err := tx.Item.Delete().Where(itemmodal.IDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete items: %w", err)
		}

		return nil
	})
}

func (s *Service) subtree(ctx context.Context, root uuid.UUID) ([]uuid.UUID, error) {
	ids := []uuid.UUID{root}
	for frontier := ids; len(frontier) > 0; {
		children, err := s.store.Item.Query().
			Where(itemmodal.ParentIDIn(frontier...), itemmodal.IDNotIn(ids...)).
			IDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query child items: %w", err)
		}
		ids = append(ids, children...)
		frontier = children
	}

	return ids, nil
}
