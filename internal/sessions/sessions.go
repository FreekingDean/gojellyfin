package sessions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/store"
	apikeymodel "github.com/FreekingDean/gojellyfin/internal/store/apikey"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
	policymodel "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

type (
	Session     = store.SessionModel
	MediaSource = store.ItemSourceModel
	Device      = store.DeviceModel
	User        = store.UserModel
)

type EndingHook func(ctx context.Context, sessionIDs []uuid.UUID)

type Service struct {
	store    *store.Client
	activity *activity.Service
	ending   []EndingHook
	now      func() time.Time
}

func (s *Service) OnEnding(hook EndingHook) {
	s.ending = append(s.ending, hook)
}

func (s *Service) end(ctx context.Context, sessionIDs []uuid.UUID) {
	if len(sessionIDs) == 0 {
		return
	}

	for _, hook := range s.ending {
		hook(ctx, sessionIDs)
	}
}

func New(client *store.Client, activity *activity.Service) *Service {
	return &Service{store: client, activity: activity, now: time.Now}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, token string, device Device) (*Session, error) {
	admitted, err := s.store.User.Query().
		Where(usermodel.ID(userID), admittedNow(s.now())).
		Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check the user's access schedule: %w", err)
	}
	if !admitted {
		return nil, ErrOutsideSchedule
	}

	if err := s.insert(ctx, userID, token, device, nil); err != nil {
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

func (s *Service) insert(ctx context.Context, userID uuid.UUID, token string, device Device, apiKeyID *uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
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
			SetNillableAPIKeyID(apiKeyID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}

		return nil
	})
}

func (s *Service) ByToken(ctx context.Context, token string) (*Session, error) {
	session, err := s.byToken(ctx, token)
	if !store.IsNotFound(err) {
		return session, err
	}

	opened, keyErr := s.openKey(ctx, token)
	if keyErr != nil {
		return nil, keyErr
	}
	if !opened {
		return nil, err
	}

	return s.byToken(ctx, token)
}

func (s *Service) byToken(ctx context.Context, token string) (*Session, error) {
	now := s.now()
	session, err := s.store.Session.Query().
		Where(
			sessionmodel.AccessToken(token),
			sessionmodel.RevokedAtIsNil(),
			sessionmodel.HasUserWith(usermodel.Not(usermodel.HasPolicyWith(policymodel.IsDisabled(true)))),
			sessionmodel.HasUserWith(admittedNow(now)),
			sessionmodel.Or(
				sessionmodel.APIKeyIDIsNil(),
				sessionmodel.HasAPIKeyWith(liveKey(now)...),
			),
		).
		WithUser().
		WithDevice().
		OnlyModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query session by token: %w", err)
	}

	return session, nil
}

func liveKey(now time.Time) []predicate.ApiKey {
	return []predicate.ApiKey{
		apikeymodel.Or(apikeymodel.RevokedAtIsNil(), apikeymodel.RevokedAtGT(now)),
		apikeymodel.HasUserWith(usermodel.HasPolicyWith(policymodel.IsAdministrator(true), policymodel.IsDisabled(false))),
	}
}

func (s *Service) openKey(ctx context.Context, token string) (bool, error) {
	key, err := s.store.ApiKey.Query().
		Where(append(liveKey(s.now()), apikeymodel.AccessToken(token))...).
		Only(ctx)
	if store.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query api key by token: %w", err)
	}

	device := Device{ClientID: KeyClientID(key.ID), Name: key.AppName, AppName: key.AppName}
	if err := s.insert(ctx, *key.UserID, token, device, &key.ID); err != nil && !store.IsConstraintError(err) {
		return false, err
	}

	return true, nil
}

func KeyClientID(keyID uuid.UUID) string {
	return "apikey-" + keyID.String()
}

func (s *Service) List(ctx context.Context) ([]*Session, error) {
	sessions, err := s.store.Session.Query().
		Where(sessionmodel.RevokedAtIsNil()).
		WithUser().
		WithDevice().
		WithNowPlaying(func(query *store.ItemQuery) {
			query.WithParent(func(query *store.ItemQuery) {
				query.WithParent()
			})
		}).
		WithPlannedSource(func(query *store.ItemSourceQuery) {
			query.WithStreams(func(query *store.MediaStreamQuery) {
				query.Order(streammodel.ByIndex())
			})
		}).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

func (s *Service) RevokeForUser(ctx context.Context, userID uuid.UUID) error {
	live := []predicate.Session{
		sessionmodel.HasUserWith(usermodel.ID(userID)),
		sessionmodel.RevokedAtIsNil(),
		sessionmodel.APIKeyIDIsNil(),
	}

	ids, err := s.store.Session.Query().Where(live...).IDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to find sessions for user: %w", err)
	}

	s.end(ctx, ids)

	if _, err := s.store.Session.Update().Where(live...).SetRevokedAt(time.Now()).Save(ctx); err != nil {
		return fmt.Errorf("failed to revoke sessions for user: %w", err)
	}

	return nil
}

func (s *Service) DeleteByToken(ctx context.Context, token string) error {
	session, err := s.ByToken(ctx, token)
	if err != nil && !store.IsNotFound(err) {
		return err
	}

	if session != nil {
		s.end(ctx, []uuid.UUID{session.ID})
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
