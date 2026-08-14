package env

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, name := range []string{
		"DATABASE_URL", "PUBLISHED_SERVER_URL", "CORS_ORIGINS",
		"TRANSCODER_JOBS", "TRANSCODER_STALL_TIMEOUT",
	} {
		t.Setenv(name, "")
	}

	config, err := Load()
	if err != nil {
		t.Fatalf("an empty environment failed to load: %v", err)
	}

	if !strings.HasPrefix(config.DatabaseURL, "postgres://") {
		t.Errorf("DatabaseURL = %q, want the development default", config.DatabaseURL)
	}
	if config.PublishedServerURL != "" {
		t.Error("an address is advertised that the server cannot confirm")
	}
	if len(config.CORSOrigins) != 0 {
		t.Error("origins were invented")
	}
}

// Silently falling back to the default is how a typo in a manifest becomes a
// capacity problem nobody can explain.
func TestLoadRefusesMalformedValues(t *testing.T) {
	t.Run("jobs", func(t *testing.T) {
		t.Setenv("TRANSCODER_JOBS", "lots")
		if _, err := Load(); err == nil {
			t.Error("a non numeric TRANSCODER_JOBS was accepted")
		}
	})

	t.Run("stall timeout", func(t *testing.T) {
		t.Setenv("TRANSCODER_STALL_TIMEOUT", "30")
		if _, err := Load(); err == nil {
			t.Error("a unitless TRANSCODER_STALL_TIMEOUT was accepted")
		}
	})

	t.Run("negative jobs", func(t *testing.T) {
		t.Setenv("TRANSCODER_JOBS", "-2")
		if _, err := Load(); err == nil {
			t.Error("a negative TRANSCODER_JOBS was accepted")
		}
	})
}

func TestLoadReadsTheEnvironment(t *testing.T) {
	t.Setenv("CORS_ORIGINS", "https://one.example, https://two.example ")
	t.Setenv("TRANSCODER_JOBS", "4")
	t.Setenv("TRANSCODER_STALL_TIMEOUT", "45s")

	config, err := Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(config.CORSOrigins) != 2 || config.CORSOrigins[1] != "https://two.example" {
		t.Errorf("CORSOrigins = %v, want both trimmed", config.CORSOrigins)
	}
	if config.Transcoder.Jobs != 4 {
		t.Errorf("Jobs = %d, want 4", config.Transcoder.Jobs)
	}
	if config.Transcoder.StallTimeout != 45*time.Second {
		t.Errorf("StallTimeout = %v, want 45s", config.Transcoder.StallTimeout)
	}
}
