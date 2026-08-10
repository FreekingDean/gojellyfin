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

func LastUser(device *Device) *User {
	for _, session := range device.Edges.Sessions {
		if session.Edges.User != nil {
			return session.Edges.User
		}
	}

	return nil
}
