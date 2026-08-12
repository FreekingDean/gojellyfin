package sessions

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"

	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
)

// Newest activity first, with each device's sessions ordered the same way so
// the first one carries the user that last used it.
func (s *Service) Devices(ctx context.Context) ([]*Device, error) {
	devices, err := s.store.Device.Query().
		Order(devicemodal.ByLastActivityAt(sql.OrderDesc())).
		WithSessions(func(query *store.SessionQuery) {
			query.Order(sessionmodal.ByLastActivityAt(sql.OrderDesc())).WithUser()
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	return devices, nil
}

func (s *Service) DeviceByClientID(ctx context.Context, clientID string) (*Device, error) {
	device, err := s.store.Device.Query().
		Where(devicemodal.ClientID(clientID)).
		WithSessions(func(query *store.SessionQuery) {
			query.Order(sessionmodal.ByLastActivityAt(sql.OrderDesc())).WithUser()
		}).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query device by client id: %w", err)
	}

	return device, nil
}

func (s *Service) RenameDevice(ctx context.Context, clientID string, customName *string) error {
	_, err := s.store.Device.Update().
		Where(devicemodal.ClientID(clientID)).
		SetNillableCustomName(customName).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to rename device: %w", err)
	}

	return nil
}

func (s *Service) RemoveDevice(ctx context.Context, clientID string) error {
	if _, err := s.store.Device.Delete().Where(devicemodal.ClientID(clientID)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	return nil
}

func LastUser(device *Device) *User {
	for _, session := range device.Edges.Sessions {
		if session.Edges.User != nil {
			return session.Edges.User
		}
	}

	return nil
}
