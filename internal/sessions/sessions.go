package sessions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

type (
	Session = store.Session
	Device  = store.Device
	User    = store.User
)

type DeviceInfo struct {
	ID         string
	Name       string
	AppName    string
	AppVersion string
}

type Service struct {
	store    *store.Client
	activity *activity.Service
}

func New(client *store.Client, activity *activity.Service) *Service {
	return &Service{store: client, activity: activity}
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

	session, err := s.ByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	s.activity.Record(ctx, activity.Event{
		Name:          fmt.Sprintf("%s has been authenticated", session.Edges.User.Username),
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
			sessionmodal.AccessToken(token),
			sessionmodal.RevokedAtIsNil(),
			sessionmodal.HasUserWith(usermodal.Not(usermodal.HasPolicyWith(policymodal.IsDisabled(true)))),
		).
		WithUser().
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

func (s *Service) RevokeForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.store.Session.Update().
		Where(
			sessionmodal.HasUserWith(usermodal.ID(userID)),
			sessionmodal.RevokedAtIsNil(),
		).
		SetRevokedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to revoke sessions for user: %w", err)
	}

	return nil
}

func (s *Service) DeleteByToken(ctx context.Context, token string) error {
	session, err := s.ByToken(ctx, token)
	if err != nil && !store.IsNotFound(err) {
		return err
	}

	if _, err := s.store.Session.Delete().Where(sessionmodal.AccessToken(token)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete session by token: %w", err)
	}

	if session != nil {
		s.activity.Record(ctx, activity.Event{
			Name:          fmt.Sprintf("%s has disconnected", session.Edges.User.Username),
			Kind:          activity.KindSessionEnded,
			ShortOverview: session.Edges.Device.Name,
			Severity:      activity.SeverityInformation,
			UserID:        &session.Edges.User.ID,
		})
	}

	return nil
}
