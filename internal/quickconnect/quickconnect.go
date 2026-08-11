package quickconnect

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

const Enabled = true

const (
	defaultExpiry = 5 * time.Minute
	codeSpace     = 1_000_000
	codeAttempts  = 10
)

var (
	ErrUnknownSecret     = errors.New("unknown quick connect secret")
	ErrUnknownCode       = errors.New("unknown quick connect code")
	ErrAlreadyAuthorized = errors.New("quick connect code is already authorized")
	ErrNotAuthorized     = errors.New("quick connect request is not authorized")
	ErrNoCode            = errors.New("no unused quick connect code available")
)

type Request struct {
	Secret       string
	Code         string
	Device       sessions.DeviceInfo
	AddedAt      time.Time
	AuthorizedBy uuid.UUID
}

func (r Request) Authenticated() bool {
	return r.AuthorizedBy != uuid.Nil
}

type Service struct {
	Expiry time.Duration

	mu       sync.Mutex
	requests map[string]*Request
}

func New() *Service {
	return &Service{Expiry: defaultExpiry, requests: make(map[string]*Request)}
}

func (s *Service) Initiate(device sessions.DeviceInfo) (Request, error) {
	secret, err := auth.NewToken()
	if err != nil {
		return Request{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()

	code, err := s.unusedCode()
	if err != nil {
		return Request{}, err
	}

	request := &Request{Secret: secret, Code: code, Device: device, AddedAt: time.Now()}
	s.requests[secret] = request

	return *request, nil
}

func (s *Service) Pending(secret string) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()

	request, ok := s.requests[secret]
	if !ok {
		return Request{}, ErrUnknownSecret
	}

	return *request, nil
}

func (s *Service) Authorize(code string, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()

	for _, request := range s.requests {
		if request.Code != code {
			continue
		}
		if request.Authenticated() {
			return ErrAlreadyAuthorized
		}
		request.AuthorizedBy = userID

		return nil
	}

	return ErrUnknownCode
}

func (s *Service) Redeem(secret string) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()

	request, ok := s.requests[secret]
	if !ok {
		return uuid.Nil, ErrUnknownSecret
	}
	if !request.Authenticated() {
		return uuid.Nil, ErrNotAuthorized
	}
	delete(s.requests, secret)

	return request.AuthorizedBy, nil
}

func (s *Service) expire() {
	cutoff := time.Now().Add(-s.Expiry)
	for secret, request := range s.requests {
		if request.AddedAt.Before(cutoff) {
			delete(s.requests, secret)
		}
	}
}

func (s *Service) unusedCode() (string, error) {
	for range codeAttempts {
		n, err := rand.Int(rand.Reader, big.NewInt(codeSpace))
		if err != nil {
			return "", err
		}

		code := fmt.Sprintf("%06d", n.Int64())
		if !s.codeInUse(code) {
			return code, nil
		}
	}

	return "", ErrNoCode
}

func (s *Service) codeInUse(code string) bool {
	for _, request := range s.requests {
		if request.Code == code {
			return true
		}
	}

	return false
}
