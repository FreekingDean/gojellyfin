package items

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	datamodal "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
)

type Datum = store.UserItemData

// A user who has never touched an item still needs a datum to report.
func (s *Service) UserItemDatum(ctx context.Context, userID, itemID uuid.UUID) (*Datum, error) {
	datum, err := s.store.UserItemData.Query().
		Where(datamodal.UserID(userID), datamodal.ItemID(itemID)).
		Only(ctx)
	if store.IsNotFound(err) {
		return &Datum{UserID: userID, ItemID: itemID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user item data: %w", err)
	}

	return datum, nil
}

func (s *Service) ListUserItemData(ctx context.Context, userID uuid.UUID, itemIDs []uuid.UUID) (map[uuid.UUID]*Datum, error) {
	data := make(map[uuid.UUID]*Datum, len(itemIDs))
	if len(itemIDs) == 0 {
		return data, nil
	}

	rows, err := s.store.UserItemData.Query().
		Where(datamodal.UserID(userID), datamodal.ItemIDIn(itemIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list user item data: %w", err)
	}

	for _, row := range rows {
		data[row.ItemID] = row
	}

	return data, nil
}

func (s *Service) SaveUserItemDatum(ctx context.Context, datum *Datum) error {
	create := s.store.UserItemData.Create().
		SetUserID(datum.UserID).
		SetItemID(datum.ItemID).
		SetPlayed(datum.Played).
		SetPlayCount(datum.PlayCount).
		SetIsFavorite(datum.IsFavorite).
		SetPlaybackPositionTicks(datum.PlaybackPositionTicks).
		SetNillableLastPlayedAt(datum.LastPlayedAt)

	err := create.
		OnConflictColumns(datamodal.FieldUserID, datamodal.FieldItemID).
		UpdatePlayed().
		UpdatePlayCount().
		UpdateIsFavorite().
		UpdatePlaybackPositionTicks().
		UpdateLastPlayedAt().
		UpdateUpdatedAt().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save user item data: %w", err)
	}

	return nil
}
