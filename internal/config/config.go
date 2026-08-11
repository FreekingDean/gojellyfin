package config

import (
	"context"
	"encoding/json"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/configuration"
)

// Server identity, shared by every DTO that carries a ServerId.
const (
	ServerID     = "e10a32fca79342d7b8b9d96e255ce1bc"
	RootFolderID = "e9d5075a555c1cbc394eec4cef295274"
)

type Service struct {
	client *store.Client
}

func New(client *store.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Configuration(ctx context.Context, key string) (json.RawMessage, error) {
	record, err := s.client.Configuration.Query().
		Where(configuration.Key(key)).
		Only(ctx)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return record.Value, nil
}

func (s *Service) SetConfiguration(ctx context.Context, key string, value json.RawMessage) error {
	return s.client.Configuration.Create().
		SetKey(key).
		SetValue(value).
		OnConflictColumns(configuration.FieldKey).
		UpdateNewValues().
		Exec(ctx)
}
