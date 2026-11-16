package apikeys

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sessions"

	"github.com/FreekingDean/gojellyfin/internal/store"
	apikeymodel "github.com/FreekingDean/gojellyfin/internal/store/apikey"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
)

type ApiKey = store.ApiKeyModel

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Keys(ctx context.Context) ([]*ApiKey, error) {
	keys, err := s.store.ApiKey.Query().
		Order(apikeymodel.ByCreatedAt(sql.OrderDesc())).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}

	return keys, nil
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, appName, token string) (*ApiKey, error) {
	key, err := s.store.ApiKey.Create().
		SetUserID(userID).
		SetAppName(appName).
		SetAccessToken(token).
		SaveModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create api key: %w", err)
	}

	return key, nil
}

func (s *Service) Revoke(ctx context.Context, token string) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		ids, err := tx.ApiKey.Query().Where(apikeymodel.AccessToken(token)).IDs(ctx)
		if err != nil {
			return fmt.Errorf("failed to find api key: %w", err)
		}

		if _, err := tx.ApiKey.Delete().Where(apikeymodel.IDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete api key: %w", err)
		}

		clientIDs := make([]string, 0, len(ids))
		for _, id := range ids {
			clientIDs = append(clientIDs, sessions.KeyClientID(id))
		}
		if _, err := tx.Device.Delete().Where(devicemodel.ClientIDIn(clientIDs...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete the api key's device: %w", err)
		}

		return nil
	})
}
