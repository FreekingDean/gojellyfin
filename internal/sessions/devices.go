package sessions

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"

	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
)

func (s *Service) Devices(ctx context.Context) ([]*Device, error) {
	devices, err := s.store.Device.Query().
		Order(devicemodel.ByLastActivityAt(sql.OrderDesc())).
		WithSessions(func(query *store.SessionQuery) {
			query.Order(sessionmodel.ByLastActivityAt(sql.OrderDesc())).WithUser()
		}).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	return devices, nil
}

func (s *Service) DeviceByClientID(ctx context.Context, clientID string) (*Device, error) {
	device, err := s.store.Device.Query().
		Where(devicemodel.ClientID(clientID)).
		WithSessions(func(query *store.SessionQuery) {
			query.Order(sessionmodel.ByLastActivityAt(sql.OrderDesc())).WithUser()
		}).
		OnlyModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query device by client id: %w", err)
	}

	return device, nil
}

func (s *Service) RenameDevice(ctx context.Context, clientID string, customName *string) error {
	_, err := s.store.Device.Update().
		Where(devicemodel.ClientID(clientID)).
		SetNillableCustomName(customName).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to rename device: %w", err)
	}

	return nil
}

func (s *Service) RemoveDevice(ctx context.Context, clientID string) error {
	if _, err := s.store.Device.Delete().Where(devicemodel.ClientID(clientID)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	return nil
}

func LastUser(device *Device) *User {
	for _, session := range device.Sessions {
		if session.User != nil {
			return session.User
		}
	}

	return nil
}
