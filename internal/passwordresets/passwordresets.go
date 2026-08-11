package passwordresets

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
)

const DefaultTTL = 15 * time.Minute

type Pin struct {
	Value   string
	Expires time.Time
}

type pending struct {
	userID  uuid.UUID
	expires time.Time
}

type Service struct {
	ttl     time.Duration
	mu      sync.Mutex
	pending map[string]pending
}

func New(ttl time.Duration) *Service {
	return &Service{ttl: ttl, pending: make(map[string]pending)}
}

func (s *Service) Expiration() time.Time {
	return time.Now().Add(s.ttl)
}

func (s *Service) Start(userID uuid.UUID) (Pin, error) {
	value, err := auth.NewToken()
	if err != nil {
		return Pin{}, err
	}
	pin := Pin{Value: value, Expires: s.Expiration()}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for issued, entry := range s.pending {
		if entry.userID == userID || now.After(entry.expires) {
			delete(s.pending, issued)
		}
	}
	s.pending[pin.Value] = pending{userID: userID, expires: pin.Expires}

	return pin, nil
}

func (s *Service) Redeem(value string) (uuid.UUID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.pending[value]
	delete(s.pending, value)
	if !ok || time.Now().After(entry.expires) {
		return uuid.Nil, false
	}

	return entry.userID, true
}
