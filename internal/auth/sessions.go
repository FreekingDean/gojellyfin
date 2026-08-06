package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CreateSession(ctx context.Context, session *Session) error {
	return s.db.WithContext(ctx).Create(session).Error
}

func (s *Service) SessionByToken(ctx context.Context, token string) (*Session, error) {
	var session Session
	if err := s.db.WithContext(ctx).First(&session, "access_token = ?", token).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *Service) ListSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := s.db.WithContext(ctx).Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

func (s *Service) DeleteSessionByToken(ctx context.Context, token string) error {
	return s.db.WithContext(ctx).Delete(&Session{}, "access_token = ?", token).Error
}

// One row per device rather than per session, newest activity first.
func (s *Service) Devices(ctx context.Context) ([]Session, error) {
	var sessions []Session
	err := s.db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (device_id) * FROM sessions
			WHERE device_id <> ''
			ORDER BY device_id, last_activity_date DESC`).
		Scan(&sessions).Error

	return sessions, err
}
