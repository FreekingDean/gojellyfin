package env

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDatabaseURL = "postgres://localhost:5432/test?sslmode=disable"

var settings = []string{
	"DATABASE_URL",
	"HTTP_PORT",
	"PUBLISHED_SERVER_URL",
	"CORS_ORIGINS",
	"TRANSCODER_JOBS",
	"TRANSCODER_STALL_TIMEOUT",
	"TEMPORAL_HOSTPORT",
	"TEMPORAL_NAMESPACE",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"TMDB_API_KEY",
}

func setEnvironment(t *testing.T, env map[string]string) {
	t.Helper()

	for _, name := range settings {
		t.Setenv(name, env[name])
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "an otherwise empty environment invents nothing",
			env:  map[string]string{"DATABASE_URL": testDatabaseURL},
			want: Config{
				DatabaseURL: testDatabaseURL,
				HTTPPort:    defaultHTTPPort,
				CORSOrigins: []string{},
			},
		},
		{
			name: "every setting comes back as the environment spelled it",
			env: map[string]string{
				"DATABASE_URL":                testDatabaseURL,
				"HTTP_PORT":                   "9000",
				"PUBLISHED_SERVER_URL":        "https://jellyfin.example",
				"CORS_ORIGINS":                "https://one.example, https://two.example ",
				"TRANSCODER_JOBS":             "4",
				"TRANSCODER_STALL_TIMEOUT":    "45s",
				"TEMPORAL_HOSTPORT":           "temporal:7233",
				"TEMPORAL_NAMESPACE":          "gojellyfin_production",
				"OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318",
				"TMDB_API_KEY":                "not-a-real-key",
			},
			want: Config{
				DatabaseURL:        testDatabaseURL,
				HTTPPort:           9000,
				PublishedServerURL: "https://jellyfin.example",
				CORSOrigins:        []string{"https://one.example", "https://two.example"},
				Transcoder:         Transcoder{Jobs: 4, StallTimeout: 45 * time.Second},
				Temporal:           Temporal{HostPort: "temporal:7233", Namespace: "gojellyfin_production"},
				Tracing:            Tracing{OTLPEndpoint: "http://collector:4318"},
				TMDB:               TMDB{APIKey: "not-a-real-key"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setEnvironment(t, test.env)

			config, err := Load()

			require.NoError(t, err)
			assert.Equal(t, test.want, config)
		})
	}
}

func TestLoadRefuses(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "an unset DATABASE_URL",
			env:  map[string]string{},
		},
		{
			name: "a non numeric TRANSCODER_JOBS",
			env:  map[string]string{"DATABASE_URL": testDatabaseURL, "TRANSCODER_JOBS": "lots"},
		},
		{
			name: "a negative TRANSCODER_JOBS",
			env:  map[string]string{"DATABASE_URL": testDatabaseURL, "TRANSCODER_JOBS": "-2"},
		},
		{
			name: "a unitless TRANSCODER_STALL_TIMEOUT",
			env:  map[string]string{"DATABASE_URL": testDatabaseURL, "TRANSCODER_STALL_TIMEOUT": "30"},
		},
		{
			name: "a non numeric HTTP_PORT",
			env:  map[string]string{"DATABASE_URL": testDatabaseURL, "HTTP_PORT": "http"},
		},
		{
			name: "an HTTP_PORT above 65535",
			env:  map[string]string{"DATABASE_URL": testDatabaseURL, "HTTP_PORT": "70000"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setEnvironment(t, test.env)

			_, err := Load()

			assert.Error(t, err)
		})
	}
}
