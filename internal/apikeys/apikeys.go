package apikeys

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"

	"github.com/FreekingDean/gojellyfin/internal/store"
	apikeymodal "github.com/FreekingDean/gojellyfin/internal/store/apikey"
)

type ApiKey = store.ApiKey

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Keys(ctx context.Context) ([]*ApiKey, error) {
	keys, err := s.store.ApiKey.Query().
		Order(apikeymodal.ByCreatedAt(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}

	return keys, nil
}

func (s *Service) Create(ctx context.Context, appName, token string) (*ApiKey, error) {
	key, err := s.store.ApiKey.Create().
		SetAppName(appName).
		SetAccessToken(token).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create api key: %w", err)
	}

	return key, nil
}

func (s *Service) Revoke(ctx context.Context, token string) error {
	if _, err := s.store.ApiKey.Delete().
		Where(apikeymodal.AccessToken(token)).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete api key: %w", err)
	}

	return nil
}
