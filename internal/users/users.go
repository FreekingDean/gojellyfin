package users

import (
	"context"
	"time"

	"github.com/google/uuid"

	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/store"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

type User struct {
	ID               uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
	Name             string
	Username         string `gorm:"uniqueIndex"`
	PasswordHash     string
	IsAdministrator  bool
	Configuration    store.JSON
	Policy           store.JSON
	LastLoginDate    *time.Time
	LastActivityDate *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (s *Service) CreateUser(ctx context.Context, user *User) error {
	return s.db.WithContext(ctx).Create(user).Error
}

func (s *Service) User(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) UserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, "username = ?", username).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) Users(ctx context.Context) ([]User, error) {
	var users []User
	if err := s.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (s *Service) UpdateUser(ctx context.Context, user *User) error {
	return s.db.WithContext(ctx).Save(user).Error
}

func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&User{}, "id = ?", id).Error
}

func (s *Service) TouchLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now()

	return s.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{"last_login_date": now, "last_activity_date": now}).Error
}

func (s *Service) UserName(ctx context.Context, id uuid.UUID) string {
	user, err := s.User(ctx, id)
	if err != nil {
		return ""
	}

	return user.Name
}
