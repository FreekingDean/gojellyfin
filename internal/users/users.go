package users

import (
	"context"
	"time"

	"github.com/google/uuid"

	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/store"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
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

type Session struct {
	ID               uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()"`
	UserID           uuid.UUID `gorm:"type:uuid;index"`
	AccessToken      string    `gorm:"uniqueIndex"`
	DeviceID         string
	DeviceName       string
	Client           string
	AppVersion       string
	LastActivityDate time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (s *Store) CreateUser(ctx context.Context, user *User) error {
	return s.db.WithContext(ctx).Create(user).Error
}

func (s *Store) User(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Store) UserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, "username = ?", username).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Store) Users(ctx context.Context) ([]User, error) {
	var users []User
	if err := s.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (s *Store) UpdateUser(ctx context.Context, user *User) error {
	return s.db.WithContext(ctx).Save(user).Error
}

func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&User{}, "id = ?", id).Error
}

func (s *Store) TouchLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now()

	return s.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{"last_login_date": now, "last_activity_date": now}).Error
}

func (s *Store) CreateSession(ctx context.Context, session *Session) error {
	return s.db.WithContext(ctx).Create(session).Error
}

func (s *Store) SessionByToken(ctx context.Context, token string) (*Session, error) {
	var session Session
	if err := s.db.WithContext(ctx).First(&session, "access_token = ?", token).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *Store) ListSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := s.db.WithContext(ctx).Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

func (s *Store) DeleteSessionByToken(ctx context.Context, token string) error {
	return s.db.WithContext(ctx).Delete(&Session{}, "access_token = ?", token).Error
}

func (s *Store) UserName(ctx context.Context, id uuid.UUID) string {
	user, err := s.User(ctx, id)
	if err != nil {
		return ""
	}

	return user.Name
}
