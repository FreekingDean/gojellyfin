package users

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/entities"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	configurationmodal "github.com/FreekingDean/gojellyfin/internal/store/userconfiguration"
	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

type (
	User          = store.User
	Configuration = store.UserConfiguration
	Policy        = store.UserPolicy

	SubtitleMode   = configurationmodal.SubtitleMode
	SyncPlayAccess = policymodal.SyncPlayAccess
	AccessSchedule = entities.AccessSchedule
)

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) CreateUser(ctx context.Context, name, passwordHash string, isAdministrator bool) (*User, error) {
	var id uuid.UUID
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		user, err := tx.User.Create().
			SetName(name).
			SetUsername(name).
			SetPasswordHash(passwordHash).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
		id = user.ID

		if _, err := tx.UserConfiguration.Create().SetUser(user).Save(ctx); err != nil {
			return fmt.Errorf("failed to create user configuration: %w", err)
		}

		_, err = tx.UserPolicy.Create().
			SetUser(user).
			SetIsAdministrator(isAdministrator).
			SetEnableCollectionManagement(isAdministrator).
			SetEnableContentDeletion(isAdministrator).
			SetEnableRemoteControlOfOtherUsers(isAdministrator).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create user policy: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.User(ctx, id)
}

func (s *Service) EnsureUser(ctx context.Context, name, passwordHash string, isAdministrator bool) (*User, error) {
	user, err := s.query().Where(usermodal.Username(name)).Only(ctx)
	if store.IsNotFound(err) {
		return s.CreateUser(ctx, name, passwordHash, isAdministrator)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user by username: %w", err)
	}
	if err := s.SetPassword(ctx, user.ID, passwordHash); err != nil {
		return nil, err
	}

	err = s.UpdatePolicy(user.ID).
		SetIsAdministrator(isAdministrator).
		SetEnableCollectionManagement(isAdministrator).
		SetEnableContentDeletion(isAdministrator).
		SetEnableRemoteControlOfOtherUsers(isAdministrator).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to set user policy: %w", err)
	}

	return s.User(ctx, user.ID)
}

func (s *Service) User(ctx context.Context, id uuid.UUID) (*User, error) {
	user, err := s.query().Where(usermodal.ID(id)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	return user, nil
}

func (s *Service) UserByUsername(ctx context.Context, username string) (*User, error) {
	user, err := s.query().Where(usermodal.Username(username)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query user by username: %w", err)
	}

	return user, nil
}

func (s *Service) FirstUser(ctx context.Context) (*User, error) {
	user, err := s.query().Order(store.Asc(usermodal.FieldCreatedAt)).First(ctx)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query the first user: %w", err)
	}

	return user, nil
}

func (s *Service) HasUsers(ctx context.Context) (bool, error) {
	exists, err := s.store.User.Query().Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to count users: %w", err)
	}

	return exists, nil
}

func (s *Service) Users(ctx context.Context) ([]*User, error) {
	users, err := s.query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

func (s *Service) Rename(ctx context.Context, id uuid.UUID, name string) error {
	if err := s.store.User.UpdateOneID(id).SetName(name).Exec(ctx); err != nil {
		return fmt.Errorf("failed to rename user: %w", err)
	}

	return nil
}

func (s *Service) SetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	if err := s.store.User.UpdateOneID(id).SetPasswordHash(passwordHash).Exec(ctx); err != nil {
		return fmt.Errorf("failed to set user password: %w", err)
	}

	return nil
}

// The caller chains the fields it wants changed; every field has a
// SetNillable form, which is the shape the api sends them in.
func (s *Service) UpdateConfiguration(id uuid.UUID) *store.UserConfigurationUpdate {
	return s.store.UserConfiguration.Update().
		Where(configurationmodal.HasUserWith(usermodal.ID(id)))
}

func (s *Service) UpdatePolicy(id uuid.UUID) *store.UserPolicyUpdate {
	return s.store.UserPolicy.Update().
		Where(policymodal.HasUserWith(usermodal.ID(id)))
}

func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if err := s.store.User.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func (s *Service) TouchLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	err := s.store.User.UpdateOneID(id).
		SetLastLoginAt(now).
		SetLastActivityAt(now).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to touch user login: %w", err)
	}

	return nil
}

func (s *Service) query() *store.UserQuery {
	return s.store.User.Query().WithConfiguration().WithPolicy()
}
