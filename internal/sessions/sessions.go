package sessions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
)

type (
	Session = store.SessionModel
	Device  = store.DeviceModel
	User    = store.UserModel
)

type Service struct {
	store    *store.Client
	activity *activity.Service
}

func New(client *store.Client, activity *activity.Service) *Service {
	return &Service{store: client, activity: activity}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, token string, device Device) (*Session, error) {
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		now := time.Now()
		deviceID, err := tx.Device.Create().
			SetClientID(device.ClientID).
			SetName(device.Name).
			SetAppName(device.AppName).
			SetAppVersion(device.AppVersion).
			SetSupportsMediaControl(false).
			SetSupportsPersistentIdentifier(false).
			SetLastActivityAt(now).
			OnConflictColumns(devicemodel.FieldClientID).
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

	session, err := s.ByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	s.activity.Record(ctx, activity.Entry{
		Name:          fmt.Sprintf("%s has been authenticated", session.User.Username),
		Kind:          activity.KindAuthenticationSucceeded,
		ShortOverview: device.Name,
		Severity:      activity.SeverityInformation,
		UserID:        &userID,
	})

	return session, nil
}

func (s *Service) ByToken(ctx context.Context, token string) (*Session, error) {
	session, err := s.store.Session.Query().
		Where(
			sessionmodel.AccessToken(token),
			sessionmodel.RevokedAtIsNil(),
		).
		WithUser().
		WithDevice().
		OnlyModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query session by token: %w", err)
	}

	return session, nil
}

func (s *Service) List(ctx context.Context) ([]*Session, error) {
	sessions, err := s.store.Session.Query().
		Where(sessionmodel.RevokedAtIsNil()).
		WithUser().
		WithDevice().
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

func (s *Service) DeleteByToken(ctx context.Context, token string) error {
	session, err := s.ByToken(ctx, token)
	if err != nil && !store.IsNotFound(err) {
		return err
	}

	if _, err := s.store.Session.Delete().Where(sessionmodel.AccessToken(token)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete session by token: %w", err)
	}

	if session != nil {
		s.activity.Record(ctx, activity.Entry{
			Name:          fmt.Sprintf("%s has disconnected", session.User.Username),
			Kind:          activity.KindSessionEnded,
			ShortOverview: session.Device.Name,
			Severity:      activity.SeverityInformation,
			UserID:        session.UserID,
		})
	}

	return nil
}
