package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

var (
	clientParsable = regexp.MustCompile(`^\d+\.\d+\.\d+(\.\d+)?$`)
	semver         = regexp.MustCompile(`^\d+\.\d+\.\d+\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*$`)
)

func TestService_Version_StaysParsableByClients(t *testing.T) {
	if got := New(env.Config{}).Version(); !clientParsable.MatchString(got) {
		t.Errorf("Version() = %q, which the Kotlin SDK's ServerVersion.fromString refuses", got)
	}
}

func TestService_DetailedVersion(t *testing.T) {
	got := New(env.Config{}).DetailedVersion()

	t.Run("leads with the Jellyfin API version", func(t *testing.T) {
		if !strings.HasPrefix(got, JellyfinVersion+"+") {
			t.Errorf("DetailedVersion() = %q, want it to start %q", got, JellyfinVersion+"+")
		}
	})

	t.Run("names this build of gojellyfin", func(t *testing.T) {
		if want := "+gojellyfin." + buildVersion; !strings.HasSuffix(got, want) {
			t.Errorf("DetailedVersion() = %q, want it to end %q", got, want)
		}
	})

	t.Run("is semver whose build metadata leaves precedence alone", func(t *testing.T) {
		if !semver.MatchString(got) {
			t.Errorf("DetailedVersion() = %q, want semver with build metadata", got)
		}
	})
}

func TestService_PackageName(t *testing.T) {
	name := New(env.Config{}).PackageName()

	t.Run("names this build of gojellyfin", func(t *testing.T) {
		if !strings.HasPrefix(name, "gojellyfin ") {
			t.Errorf("PackageName() = %q, want a gojellyfin build", name)
		}
	})

	t.Run("does not answer the Jellyfin API version", func(t *testing.T) {
		if strings.Contains(name, JellyfinVersion) {
			t.Errorf("PackageName() = %q, want gojellyfin's own version rather than %q", name, JellyfinVersion)
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
