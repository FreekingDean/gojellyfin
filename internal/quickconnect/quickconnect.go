package quickconnect

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	requestmodel "github.com/FreekingDean/gojellyfin/internal/store/quickconnectrequest"
)

const (
	expiry    = 10 * time.Minute
	codeFloor = 100_000
	codeSpace = 900_000

	ipv4Bits          = 32
	privateIPv4Bits   = 24
	ipv6Bits          = 128
	ipv6HouseholdBits = 64
)

const redeemQuery = `
	DELETE FROM quick_connect_requests
	WHERE secret = $1 AND expires_at > now() AND authorized_by_id IS NOT NULL
	RETURNING authorized_by_id`

var ErrNotFound = errors.New("no live quick connect request")

type Request = store.QuickConnectRequestModel

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Initiate(ctx context.Context, device sessions.Device, address string) (*Request, error) {
	if _, err := s.store.QuickConnectRequest.Delete().
		Where(requestmodel.ExpiresAtLTE(time.Now())).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to sweep the expired quick connect requests: %w", err)
	}

	secret, err := auth.NewToken()
	if err != nil {
		return nil, err
	}

	n, err := rand.Int(rand.Reader, big.NewInt(codeSpace))
	if err != nil {
		return nil, err
	}

	request, err := s.store.QuickConnectRequest.Create().
		SetSecret(secret).
		SetCode(fmt.Sprint(codeFloor + n.Int64())).
		SetDeviceID(device.ClientID).
		SetDeviceName(device.Name).
		SetAppName(device.AppName).
		SetAppVersion(device.AppVersion).
		SetRemoteAddress(address).
		SetExpiresAt(time.Now().Add(expiry)).
		SaveModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create the quick connect request: %w", err)
	}

	return request, nil
}

func (s *Service) Pending(ctx context.Context, secret string) (*Request, error) {
	request, err := s.store.QuickConnectRequest.Query().
		Where(requestmodel.Secret(secret), requestmodel.ExpiresAtGT(time.Now())).
		OnlyModel(ctx)
	if store.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query the quick connect request: %w", err)
	}

	return request, nil
}

func (s *Service) Waiting(ctx context.Context) ([]*Request, error) {
	requests, err := s.store.QuickConnectRequest.Query().
		Where(requestmodel.ExpiresAtGT(time.Now()), requestmodel.AuthorizedByIDIsNil()).
		Order(requestmodel.ByCreatedAt(sql.OrderDesc())).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the waiting quick connect requests: %w", err)
	}

	return requests, nil
}

func (s *Service) Nearby(ctx context.Context, address string) ([]*Request, error) {
	waiting, err := s.Waiting(ctx)
	if err != nil {
		return nil, err
	}

	near := []*Request{}
	for _, request := range waiting {
		if SameNetwork(request.RemoteAddress, address) {
			near = append(near, request)
		}
	}

	return near, nil
}

func SameNetwork(a, b string) bool {
	first, ok := host(a)
	if !ok {
		return false
	}
	second, ok := host(b)
	if !ok || first.Is4() != second.Is4() {
		return false
	}

	bits := householdBits(first)
	if bits != householdBits(second) {
		return false
	}

	return netip.PrefixFrom(first, bits).Masked() == netip.PrefixFrom(second, bits).Masked()
}

func host(address string) (netip.Addr, bool) {
	if port, err := netip.ParseAddrPort(address); err == nil {
		return port.Addr().Unmap(), true
	}
	addr, err := netip.ParseAddr(address)

	return addr.Unmap(), err == nil
}

func householdBits(addr netip.Addr) int {
	switch {
	case addr.Is6() && addr.IsLoopback():
		return ipv6Bits
	case addr.Is6():
		return ipv6HouseholdBits
	case addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast():
		return privateIPv4Bits
	default:
		return ipv4Bits
	}
}

func (s *Service) Authorize(ctx context.Context, code string, userID uuid.UUID) error {
	authorized, err := s.store.QuickConnectRequest.Update().
		Where(requestmodel.Code(code), requestmodel.ExpiresAtGT(time.Now()), requestmodel.AuthorizedByIDIsNil()).
		SetAuthorizedByID(userID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to authorize the quick connect request: %w", err)
	}
	if authorized == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *Service) Redeem(ctx context.Context, secret string) (uuid.UUID, error) {
	rows, err := s.store.QueryContext(ctx, redeemQuery, secret)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to redeem the quick connect request: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return uuid.Nil, fmt.Errorf("failed to redeem the quick connect request: %w", err)
		}
		return uuid.Nil, ErrNotFound
	}

	var userID uuid.UUID
	if err := rows.Scan(&userID); err != nil {
		return uuid.Nil, fmt.Errorf("failed to read the redeemed quick connect request: %w", err)
	}

	return userID, nil
}
