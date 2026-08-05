package store

import (
	"context"
	"time"

	"github.com/google/uuid"
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

func (s *storeImpl) CreateSession(ctx context.Context, sess *Session) error {
	return s.db.WithContext(ctx).Create(sess).Error
}

func (s *storeImpl) GetSessionByToken(ctx context.Context, token string) (*Session, error) {
	var sess Session
	if err := s.db.WithContext(ctx).First(&sess, "access_token = ?", token).Error; err != nil {
		return nil, err
	}

	return &sess, nil
}

func (s *storeImpl) ListSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := s.db.WithContext(ctx).Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

func (s *storeImpl) UpdateSession(ctx context.Context, sess *Session) error {
	return s.db.WithContext(ctx).Save(sess).Error
}

func (s *storeImpl) DeleteSessionByToken(ctx context.Context, token string) error {
	return s.db.WithContext(ctx).Delete(&Session{}, "access_token = ?", token).Error
}
