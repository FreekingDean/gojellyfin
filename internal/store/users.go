package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
	Name             string
	Username         string `gorm:"uniqueIndex"`
	PasswordHash     string
	IsAdministrator  bool
	Configuration    JSON
	Policy           JSON
	LastLoginDate    *time.Time
	LastActivityDate *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (s *storeImpl) CreateUser(ctx context.Context, u *User) error {
	return s.db.WithContext(ctx).Create(u).Error
}

func (s *storeImpl) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	if err := s.db.WithContext(ctx).First(&u, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &u, nil
}

func (s *storeImpl) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	if err := s.db.WithContext(ctx).First(&u, "username = ?", username).Error; err != nil {
		return nil, err
	}

	return &u, nil
}

func (s *storeImpl) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := s.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (s *storeImpl) UpdateUser(ctx context.Context, u *User) error {
	return s.db.WithContext(ctx).Save(u).Error
}

func (s *storeImpl) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&User{}, "id = ?", id).Error
}

func (s *storeImpl) TouchUserLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now()

	return s.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{"last_login_date": now, "last_activity_date": now}).Error
}

func (s *storeImpl) TouchUserActivity(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Update("last_activity_date", time.Now()).Error
}
