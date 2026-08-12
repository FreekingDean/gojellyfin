package quickconnect

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

func newService(t *testing.T, expiry time.Duration) *Service {
	t.Helper()

	service := New()
	service.expiry = expiry

	return service
}

func initiate(t *testing.T, service *Service) Request {
	t.Helper()

	request, err := service.Initiate(sessions.DeviceInfo{ID: "tv", Name: "TV", AppName: "Jellyfin Web"})
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	return request
}

func TestInitiateDescribesAFreshRequest(t *testing.T) {
	service := New()

	request := initiate(t, service)

	if request.Authenticated() {
		t.Error("Authenticated = true, want a fresh request to be unauthorized")
	}
	if len(request.Code) != 6 {
		t.Errorf("Code = %q, want six digits", request.Code)
	}
	if len(request.Secret) < 32 {
		t.Errorf("Secret = %q, want an unguessable value", request.Secret)
	}
	if request.Device.ID != "tv" {
		t.Errorf("Device.ID = %q, want the initiating device", request.Device.ID)
	}
}

func TestInitiateIssuesDistinctSecretsAndCodes(t *testing.T) {
	service := New()

	first := initiate(t, service)
	second := initiate(t, service)

	if first.Secret == second.Secret {
		t.Error("Secret repeated, want a fresh secret per request")
	}
	if first.Code == second.Code {
		t.Errorf("Code = %q twice, want pending codes to be distinct", first.Code)
	}
}

func TestInitiateBoundsThePendingRequests(t *testing.T) {
	service := New()

	for range maxPending {
		initiate(t, service)
	}

	if _, err := service.Initiate(sessions.DeviceInfo{ID: "tv"}); !errors.Is(err, ErrTooManyPending) {
		t.Fatalf("err = %v, want ErrTooManyPending", err)
	}
}

func TestTheCodeIsNotThePollCredential(t *testing.T) {
	service := New()

	request := initiate(t, service)
	if err := service.Authorize(request.Code, uuid.New()); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	if _, err := service.Pending(request.Code); !errors.Is(err, ErrUnknownSecret) {
		t.Errorf("Pending(code) err = %v, want ErrUnknownSecret", err)
	}
	if _, err := service.Redeem(request.Code); !errors.Is(err, ErrUnknownSecret) {
		t.Errorf("Redeem(code) err = %v, want ErrUnknownSecret", err)
	}
}

func TestAuthorizeCannotDistinguishUnknownExpiredAndUsedCodes(t *testing.T) {
	userID := uuid.New()

	expired := newService(t, time.Nanosecond)
	expiring := initiate(t, expired)

	used := New()
	redeemed := initiate(t, used)
	if err := used.Authorize(redeemed.Code, userID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}
	if _, err := used.Redeem(redeemed.Secret); err != nil {
		t.Fatalf("failed to redeem: %v", err)
	}

	if err := New().Authorize("000000", userID); !errors.Is(err, ErrUnknownCode) {
		t.Errorf("unknown code err = %v, want ErrUnknownCode", err)
	}
	if err := expired.Authorize(expiring.Code, userID); !errors.Is(err, ErrUnknownCode) {
		t.Errorf("expired code err = %v, want ErrUnknownCode", err)
	}
	if err := used.Authorize(redeemed.Code, userID); !errors.Is(err, ErrUnknownCode) {
		t.Errorf("redeemed code err = %v, want ErrUnknownCode", err)
	}
}

func TestAuthorizeRefusesToRebindAPendingCode(t *testing.T) {
	service := New()
	userID := uuid.New()

	request := initiate(t, service)
	if err := service.Authorize(request.Code, userID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	if err := service.Authorize(request.Code, uuid.New()); !errors.Is(err, ErrAlreadyAuthorized) {
		t.Fatalf("err = %v, want ErrAlreadyAuthorized", err)
	}

	authorized, err := service.Redeem(request.Secret)
	if err != nil {
		t.Fatalf("failed to redeem: %v", err)
	}
	if authorized != userID {
		t.Errorf("authorized user = %v, want the first authorizing user %v", authorized, userID)
	}
}

func TestRedeemRefusesAnUnauthorizedRequest(t *testing.T) {
	service := New()

	request := initiate(t, service)

	if _, err := service.Redeem(request.Secret); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want ErrNotAuthorized", err)
	}
}

func TestRedeemRefusesAnExpiredRequest(t *testing.T) {
	service := newService(t, time.Nanosecond)

	request := initiate(t, service)
	if err := service.Authorize(request.Code, uuid.New()); !errors.Is(err, ErrUnknownCode) {
		t.Fatalf("err = %v, want an expired code to be unauthorizable", err)
	}

	if _, err := service.Redeem(request.Secret); !errors.Is(err, ErrUnknownSecret) {
		t.Fatalf("err = %v, want ErrUnknownSecret", err)
	}
}

func TestRedeemIsSingleUse(t *testing.T) {
	service := New()
	userID := uuid.New()

	request := initiate(t, service)
	if err := service.Authorize(request.Code, userID); err != nil {
		t.Fatalf("failed to authorize: %v", err)
	}

	authorized, err := service.Redeem(request.Secret)
	if err != nil {
		t.Fatalf("failed to redeem: %v", err)
	}
	if authorized != userID {
		t.Errorf("authorized user = %v, want %v", authorized, userID)
	}

	if _, err := service.Redeem(request.Secret); !errors.Is(err, ErrUnknownSecret) {
		t.Fatalf("err = %v, want the secret to be spent", err)
	}
	if _, err := service.Pending(request.Secret); !errors.Is(err, ErrUnknownSecret) {
		t.Fatalf("err = %v, want the secret to be spent", err)
	}
}

func TestServiceIsSafeForConcurrentUse(t *testing.T) {
	service := New()

	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()

			request, err := service.Initiate(sessions.DeviceInfo{ID: "tv"})
			if err != nil {
				t.Errorf("failed to initiate: %v", err)

				return
			}
			if _, err := service.Pending(request.Secret); err != nil {
				t.Errorf("failed to poll: %v", err)
			}
			if err := service.Authorize(request.Code, uuid.New()); err != nil {
				t.Errorf("failed to authorize: %v", err)
			}
			if _, err := service.Redeem(request.Secret); err != nil {
				t.Errorf("failed to redeem: %v", err)
			}
		}()
	}
	group.Wait()
}
