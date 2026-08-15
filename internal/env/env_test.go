package env

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

const testDatabaseURL = "postgres://localhost:5432/test?sslmode=disable"

func TestLoadDefaults(t *testing.T) {
	for _, name := range []string{
		"PUBLISHED_SERVER_URL", "CORS_ORIGINS",
		"TRANSCODER_JOBS", "TRANSCODER_STALL_TIMEOUT",
		"TEMPORAL_HOSTPORT", "TEMPORAL_NAMESPACE",
		"HTTP_PORT",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("DATABASE_URL", testDatabaseURL)

	config, err := Load()
	if err != nil {
		t.Fatalf("an otherwise empty environment failed to load: %v", err)
	}

	if config.PublishedServerURL != "" {
		t.Error("an address is advertised that the server cannot confirm")
	}
	if len(config.CORSOrigins) != 0 {
		t.Error("origins were invented")
	}
	if config.Temporal.HostPort != "" {
		t.Error("a Temporal address was invented, so background work dials on every start")
	}
	if config.Temporal.Namespace != defaultTemporalNamespace {
		t.Errorf("Namespace = %q, want %q", config.Temporal.Namespace, defaultTemporalNamespace)
	}
	if config.HTTPPort != defaultHTTPPort {
		t.Errorf("HTTPPort = %d, want %d", config.HTTPPort, defaultHTTPPort)
	}
}

// Silently falling back to the default is how a typo in a manifest becomes a
// capacity problem nobody can explain.
func TestLoadRefusesMalformedValues(t *testing.T) {
	t.Setenv("DATABASE_URL", testDatabaseURL)

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

	t.Run("port out of range", func(t *testing.T) {
		t.Setenv("HTTP_PORT", "70000")
		if _, err := Load(); err == nil {
			t.Error("an HTTP_PORT above 65535 was accepted")
		}
	})

	t.Run("port is not a number", func(t *testing.T) {
		t.Setenv("HTTP_PORT", "http")
		if _, err := Load(); err == nil {
			t.Error("a non numeric HTTP_PORT was accepted")
		}
	})
}

// An unbound key reads as unset however the manifest spells it, so a field
// added to Config without a line in keys is a knob that silently does nothing.
func TestEveryFieldIsBound(t *testing.T) {
	var walk func(reflect.Type)
	walk = func(structType reflect.Type) {
		for i := range structType.NumField() {
			field := structType.Field(i)
			tag := field.Tag.Get("mapstructure")
			if tag == ",squash" {
				walk(field.Type)
				continue
			}
			if !slices.Contains(keys, tag) {
				t.Errorf("%s is not bound: add %q to keys", field.Name, tag)
			}
		}
	}

	walk(reflect.TypeOf(Config{}))
}

// Dialing localhost because nobody said otherwise is how a process ends up
// talking to the wrong database without anyone noticing.
func TestLoadRequiresADatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Error("an unset DATABASE_URL was accepted")
	}
}

func TestLoadReadsTheEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("CORS_ORIGINS", "https://one.example, https://two.example ")
	t.Setenv("TRANSCODER_JOBS", "4")
	t.Setenv("TRANSCODER_STALL_TIMEOUT", "45s")
	t.Setenv("TEMPORAL_HOSTPORT", "temporal:7233")
	t.Setenv("TEMPORAL_NAMESPACE", "gojellyfin_production")
	t.Setenv("HTTP_PORT", "9000")

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
	if config.Temporal.HostPort != "temporal:7233" {
		t.Errorf("HostPort = %q, want temporal:7233", config.Temporal.HostPort)
	}
	if config.Temporal.Namespace != "gojellyfin_production" {
		t.Errorf("Namespace = %q, want gojellyfin_production", config.Temporal.Namespace)
	}
	if config.HTTPPort != 9000 {
		t.Errorf("HTTPPort = %d, want 9000", config.HTTPPort)
	}
}
