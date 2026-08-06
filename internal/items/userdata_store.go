package items

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Datum struct {
	ID                    uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
	UserID                uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_user_item_data_user_item"`
	ItemID                uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_user_item_data_user_item;index"`
	Played                bool
	PlayCount             int32
	IsFavorite            bool
	PlaybackPositionTicks int64
	LastPlayedDate        *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Renaming the model must not move the table.
func (Datum) TableName() string {
	return "user_item_data"
}

func (s *Server) GetUserItemDatum(ctx context.Context, userID, itemID uuid.UUID) (*Datum, error) {
	var datum Datum
	err := s.db.WithContext(ctx).First(&datum, "user_id = ? AND item_id = ?", userID, itemID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &Datum{UserID: userID, ItemID: itemID}, nil
	}
	if err != nil {
		return nil, err
	}

	return &datum, nil
}

func (s *Server) ListUserItemData(ctx context.Context, userID uuid.UUID, itemIDs []uuid.UUID) (map[uuid.UUID]Datum, error) {
	data := make(map[uuid.UUID]Datum, len(itemIDs))
	if len(itemIDs) == 0 {
		return data, nil
	}

	var rows []Datum
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND item_id IN ?", userID, itemIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		data[row.ItemID] = row
	}

	return data, nil
}

func (s *Server) SaveUserItemDatum(ctx context.Context, datum *Datum) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "item_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"played", "play_count", "is_favorite", "playback_position_ticks",
			"last_played_date", "updated_at",
		}),
	}).Create(datum).Error
}
