package apikeys

import (
	"context"

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
	return s.store.ApiKey.Query().
		Order(apikeymodal.ByCreatedAt(sql.OrderDesc())).
		All(ctx)
}

func (s *Service) Create(ctx context.Context, appName, token string) (*ApiKey, error) {
	return s.store.ApiKey.Create().
		SetAppName(appName).
		SetAccessToken(token).
		Save(ctx)
}

func (s *Service) Revoke(ctx context.Context, token string) error {
	_, err := s.store.ApiKey.Delete().
		Where(apikeymodal.AccessToken(token)).
		Exec(ctx)

	return err
}
