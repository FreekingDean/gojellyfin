package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
)

type (
	Session = store.Session
	Device  = store.Device
)

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) CreateSession(ctx context.Context, session *Session, device *Device) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		deviceID, err := tx.Device.Create().
			SetClientID(device.ClientID).
			SetName(device.Name).
			SetAppVersion(device.AppVersion).
			SetLastActivityAt(time.Now()).
			OnConflictColumns(devicemodal.FieldClientID).
			UpdateAppVersion().
			UpdateLastActivityAt().
			ID(ctx)
		if err != nil {
			return fmt.Errorf("failed to create or update device: %w", err)
		}
		_, err = tx.Session.Create().
			SetUserID(session.Edges.User.ID).
			SetAccessToken(session.AccessToken).
			SetDeviceID(deviceID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}

		return nil
	})
}

func (s *Service) SessionByToken(ctx context.Context, token string) (*Session, error) {
	session, err := s.store.Session.Query().Where(sessionmodal.AccessToken(token)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query session by token: %w", err)
	}

	if session.RevokedAt.Before(time.Now()) {
		return nil, fmt.Errorf("session is revoked")
	}

	return session, nil
}

func (s *Service) ListSessions(ctx context.Context) ([]*Session, error) {
	sessions, err := s.store.Session.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

func (s *Service) DeleteSessionByToken(ctx context.Context, token string) error {
	_, err := s.store.Session.Delete().Where(
		sessionmodal.AccessToken(token),
	).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete session by token: %w", err)
	}

	return nil
}

// One row per device rather than per session, newest activity first.
func (s *Service) Devices(ctx context.Context) ([]*Device, error) {
	devices, err := s.store.Device.Query().Order(devicemodal.ByLastActivityAt()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	return devices, err
}
