package jobs

import (
	"errors"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

// Background work is off rather than broken when no address names a server, so
// a developer running the server alone gets a server.
func TestNewClientWithoutAnAddressIsDisabled(t *testing.T) {
	client, err := NewClient(env.Config{})
	if err != nil {
		t.Fatalf("an unconfigured client failed to build: %v", err)
	}
	if client.Enabled() {
		t.Error("a client with no address reports itself enabled")
	}
}

// A namespace is only required once there is a server to dial, and it is
// refused here rather than defaulted, so a run cannot land in the wrong one.
func TestNewClientRequiresANamespace(t *testing.T) {
	_, err := NewClient(env.Config{Temporal: env.Temporal{HostPort: "temporal:7233"}})

	if !errors.Is(err, ErrNoNamespace) {
		t.Errorf("err = %v, want ErrNoNamespace", err)
	}
}
