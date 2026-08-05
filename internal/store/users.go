package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()" json:"id"`
	Name      string    `json:"name"`
	Username  string    `gorm:"uniqueIndex" json:"username"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
