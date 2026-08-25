package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func TestService_Version(t *testing.T) {
	t.Run("answers the version the vendored spec declares", func(t *testing.T) {
		want := specVersion(t)

		if got := New(env.Config{}).Version(); got != want {
			t.Errorf("Version() = %q, want %q", got, want)
		}
	})
}

func specVersion(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "spec", "jellyfin-openapi-stable.json"))
	if err != nil {
		t.Fatalf("failed to read the vendored spec: %v", err)
	}

	var doc struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("failed to parse the vendored spec: %v", err)
	}

	return doc.Info.Version
}
