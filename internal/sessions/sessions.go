package sessions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
)

type (
	Session = store.Session
	Device  = store.Device
	User    = store.User
)

// What the client told us about itself when it authenticated.
type DeviceInfo struct {
	ID         string
	Name       string
	AppName    string
	AppVersion string
}

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, token string, device DeviceInfo) (*Session, error) {
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		now := time.Now()
		deviceID, err := tx.Device.Create().
			SetClientID(device.ID).
			SetName(device.Name).
			SetAppName(device.AppName).
			SetAppVersion(device.AppVersion).
			SetSupportsMediaControl(false).
			SetSupportsPersistentIdentifier(false).
			SetLastActivityAt(now).
			OnConflictColumns(devicemodal.FieldClientID).
			UpdateName().
			UpdateAppName().
			UpdateAppVersion().
			UpdateLastActivityAt().
			ID(ctx)
		if err != nil {
			return fmt.Errorf("failed to create or update device: %w", err)
		}

		_, err = tx.Session.Create().
			SetUserID(userID).
			SetDeviceID(deviceID).
			SetAccessToken(token).
			SetLastActivityAt(now).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.ByToken(ctx, token)
}

func (s *Service) ByToken(ctx context.Context, token string) (*Session, error) {
	session, err := s.store.Session.Query().
		Where(
			sessionmodal.AccessToken(token),
			sessionmodal.RevokedAtIsNil(),
		).
		WithUser(func(user *store.UserQuery) { user.WithPolicy() }).
		WithDevice().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query session by token: %w", err)
	}

	return session, nil
}

func (s *Service) List(ctx context.Context) ([]*Session, error) {
	sessions, err := s.store.Session.Query().
		Where(sessionmodal.RevokedAtIsNil()).
		WithUser().
		WithDevice().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

func (s *Service) DeleteByToken(ctx context.Context, token string) error {
	_, err := s.store.Session.Delete().
		Where(sessionmodal.AccessToken(token)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete session by token: %w", err)
	}

	return nil
}
