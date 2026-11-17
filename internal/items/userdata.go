package items

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	datamodel "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
)

type Datum = store.UserItemDataModel

func (s *Service) UserItemDatum(ctx context.Context, userID, itemID uuid.UUID) (*Datum, error) {
	datum, err := s.store.UserItemData.Query().
		Where(datamodel.UserID(userID), datamodel.ItemID(itemID)).
		OnlyModel(ctx)
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
		Where(datamodel.UserID(userID), datamodel.ItemIDIn(itemIDs...)).
		AllModels(ctx)
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
		OnConflictColumns(datamodel.FieldUserID, datamodel.FieldItemID).
		UpdatePlayed().
		UpdatePlayCount().
		UpdateIsFavorite().
		UpdatePlaybackPositionTicks().
		UpdateLastPlayedAt().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save user item data: %w", err)
	}

	return nil
}

const playedThreshold = 0.9

func (s *Service) MarkPlayed(ctx context.Context, userID, itemID uuid.UUID, played bool) (*Datum, error) {
	return s.changeDatum(ctx, userID, itemID, func(datum *Datum) {
		datum.Played = played
		datum.PlaybackPositionTicks = 0
		if played {
			now := time.Now()
			datum.PlayCount++
			datum.LastPlayedAt = &now
		}
	})
}

func (s *Service) MarkFavorite(ctx context.Context, userID, itemID uuid.UUID, favorite bool) (*Datum, error) {
	return s.changeDatum(ctx, userID, itemID, func(datum *Datum) {
		datum.IsFavorite = favorite
	})
}

func (s *Service) RecordProgress(ctx context.Context, userID, itemID uuid.UUID, position int64) error {
	_, err := s.changeDatum(ctx, userID, itemID, func(datum *Datum) {
		now := time.Now()
		datum.PlaybackPositionTicks = position
		datum.LastPlayedAt = &now
	})

	return err
}

func (s *Service) RecordStop(ctx context.Context, userID, itemID uuid.UUID, position int64) error {
	item, err := s.ItemByID(ctx, Everyone, itemID)
	if err != nil {
		return err
	}

	_, err = s.changeDatum(ctx, userID, itemID, func(datum *Datum) {
		now := time.Now()
		datum.LastPlayedAt = &now
		datum.PlaybackPositionTicks = position
		if watched(item.RunTimeTicks, position) {
			datum.Played = true
			datum.PlayCount++
			datum.PlaybackPositionTicks = 0
		}
	})

	return err
}

func (s *Service) changeDatum(ctx context.Context, userID, itemID uuid.UUID, change func(*Datum)) (*Datum, error) {
	datum, err := s.UserItemDatum(ctx, userID, itemID)
	if err != nil {
		return nil, err
	}

	change(datum)

	return datum, s.SaveUserItemDatum(ctx, datum)
}

func watched(runTimeTicks *int64, position int64) bool {
	if runTimeTicks == nil || *runTimeTicks == 0 {
		return false
	}

	return float64(position)/float64(*runTimeTicks) >= playedThreshold
}
