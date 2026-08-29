package quickconnect

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	requestmodal "github.com/FreekingDean/gojellyfin/internal/store/quickconnectrequest"
)

const Enabled = true

const (
	defaultExpiry     = 10 * time.Minute
	codeFloor         = 100_000
	codeSpace         = 900_000
	codeAttempts      = 10
	defaultMaxPending = codeSpace / codeAttempts
)

var (
	ErrUnknownSecret     = errors.New("unknown quick connect secret")
	ErrUnknownCode       = errors.New("unknown quick connect code")
	ErrAlreadyAuthorized = errors.New("quick connect code is already authorized")
	ErrNotAuthorized     = errors.New("quick connect request is not authorized")
	ErrNoCode            = errors.New("no unused quick connect code available")
	ErrTooManyPending    = errors.New("too many pending quick connect requests")
)

type Request = store.QuickConnectRequest

type Service struct {
	store      *store.Client
	expiry     time.Duration
	maxPending int
}

func New(client *store.Client) *Service {
	return &Service{store: client, expiry: defaultExpiry, maxPending: defaultMaxPending}
}

func (s *Service) Initiate(ctx context.Context, device sessions.DeviceInfo) (*Request, error) {
	if err := s.sweep(ctx); err != nil {
		return nil, err
	}

	pending, err := s.store.QuickConnectRequest.Query().Where(live()).Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count the pending quick connect requests: %w", err)
	}
	if pending >= s.maxPending {
		return nil, ErrTooManyPending
	}

	for range codeAttempts {
		secret, err := auth.NewToken()
		if err != nil {
			return nil, err
		}

		code, err := newCode()
		if err != nil {
			return nil, err
		}

		request, err := s.store.QuickConnectRequest.Create().
			SetSecret(secret).
			SetCode(code).
			SetDeviceID(device.ID).
			SetDeviceName(device.Name).
			SetAppName(device.AppName).
			SetAppVersion(device.AppVersion).
			SetExpiresAt(time.Now().Add(s.expiry)).
			Save(ctx)
		if store.IsConstraintError(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create the quick connect request: %w", err)
		}

		return request, nil
	}

	return nil, ErrNoCode
}

func (s *Service) Pending(ctx context.Context, secret string) (*Request, error) {
	request, err := s.store.QuickConnectRequest.Query().
		Where(requestmodal.Secret(secret), live()).
		Only(ctx)
	if store.IsNotFound(err) {
		return nil, ErrUnknownSecret
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query the quick connect request: %w", err)
	}

	return request, nil
}

func (s *Service) Authorize(ctx context.Context, code string, userID uuid.UUID) error {
	authorized, err := s.store.QuickConnectRequest.Update().
		Where(requestmodal.Code(code), live(), requestmodal.AuthorizedByIDIsNil()).
		SetAuthorizedByID(userID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to authorize the quick connect request: %w", err)
	}
	if authorized > 0 {
		return nil
	}

	taken, err := s.store.QuickConnectRequest.Query().
		Where(requestmodal.Code(code), live()).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to query the quick connect request: %w", err)
	}
	if taken {
		return ErrAlreadyAuthorized
	}

	return ErrUnknownCode
}

func (s *Service) Redeem(ctx context.Context, secret string) (uuid.UUID, error) {
	var userID uuid.UUID

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		request, err := tx.QuickConnectRequest.Query().
			Where(requestmodal.Secret(secret), live()).
			Only(ctx)
		if store.IsNotFound(err) {
			return ErrUnknownSecret
		}
		if err != nil {
			return fmt.Errorf("failed to query the quick connect request: %w", err)
		}
		if request.AuthorizedByID == uuid.Nil {
			return ErrNotAuthorized
		}

		spent, err := tx.QuickConnectRequest.Delete().
			Where(requestmodal.ID(request.ID)).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to spend the quick connect request: %w", err)
		}
		if spent == 0 {
			return ErrUnknownSecret
		}

		userID = request.AuthorizedByID

		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}

func (s *Service) sweep(ctx context.Context) error {
	if _, err := s.store.QuickConnectRequest.Delete().
		Where(requestmodal.ExpiresAtLTE(time.Now())).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to sweep the expired quick connect requests: %w", err)
	}

	return nil
}

func live() predicate.QuickConnectRequest {
	return requestmodal.ExpiresAtGT(time.Now())
}

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeSpace))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", codeFloor+n.Int64()), nil
}
