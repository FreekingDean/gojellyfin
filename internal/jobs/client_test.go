package jobs

import (
	"errors"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func TestNewClient(t *testing.T) {
	t.Run("without an address is disabled", func(t *testing.T) {
		client, err := NewClient(env.Config{})
		if err != nil {
			t.Fatalf("an unconfigured client failed to build: %v", err)
		}
		if client.Enabled() {
			t.Error("a client with no address reports itself enabled")
		}
	})

	t.Run("requires a namespace", func(t *testing.T) {
		_, err := NewClient(env.Config{Temporal: env.Temporal{HostPort: "temporal:7233"}})

		if !errors.Is(err, ErrNoNamespace) {
			t.Errorf("err = %v, want ErrNoNamespace", err)
		}
	})
}
