package env

import (
	"os"
	"strings"
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
	"SERVER_ID",
	"CORS_ORIGINS",
	"TRANSCODER_JOBS",
	"TRANSCODER_STALL_TIMEOUT",
	"TEMPORAL_HOSTPORT",
	"TEMPORAL_NAMESPACE",
	"TEMPORAL_WORKER_STOP_TIMEOUT",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"TMDB_API_KEY",
	"TMDB_BASE_URL",
	"MEDIA_DIRECTORIES",
	"OIDC_ISSUER_URL",
	"OIDC_CLIENT_ID",
	"OIDC_CLIENT_SECRET",
	"OIDC_SCOPES",
	"OIDC_USERNAME_CLAIM",
	"OIDC_GROUPS_CLAIM",
	"OIDC_ADMIN_GROUP",
	"OIDC_ALLOWED_GROUP",
	"OIDC_LINK_EXISTING_USERS",
}

func setEnvironment(t *testing.T, env map[string]string) {
	t.Helper()

	for _, entry := range os.Environ() {
		if name, _, split := strings.Cut(entry, "="); split && strings.HasPrefix(name, SourceAPIKeyPrefix) {
			t.Setenv(name, "")
		}
	}
	for _, name := range settings {
		t.Setenv(name, env[name])
	}
	for name, value := range env {
		t.Setenv(name, value)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "an otherwise empty environment invents nothing",
			env:  map[string]string{"DATABASE_URL": testDatabaseURL},
			want: Config{
				DatabaseURL:      testDatabaseURL,
				HTTPPort:         defaultHTTPPort,
				CORSOrigins:      []string{},
				MediaDirectories: []string{"/"},
				SourceAPIKeys:    map[string]string{},
			},
		},
		{
			name: "every setting comes back as the environment spelled it",
			env: map[string]string{
				"DATABASE_URL":                 testDatabaseURL,
				"HTTP_PORT":                    "9000",
				"PUBLISHED_SERVER_URL":         "https://jellyfin.example",
				"SERVER_ID":                    "0123456789abcdef0123456789abcdef",
				"CORS_ORIGINS":                 "https://one.example, https://two.example ",
				"TRANSCODER_JOBS":              "4",
				"TRANSCODER_STALL_TIMEOUT":     "45s",
				"TEMPORAL_HOSTPORT":            "temporal:7233",
				"TEMPORAL_NAMESPACE":           "gojellyfin_production",
				"TEMPORAL_WORKER_STOP_TIMEOUT": "5m",
				"OTEL_EXPORTER_OTLP_ENDPOINT":  "http://collector:4318",
				"TMDB_API_KEY":                 "not-a-real-key",
				"MEDIA_DIRECTORIES":            "/media, /library ",
				"OIDC_ISSUER_URL":              "https://auth.example",
				"OIDC_CLIENT_ID":               "gojellyfin",
				"OIDC_CLIENT_SECRET":           "not-a-real-secret",
				"OIDC_SCOPES":                  "openid email",
				"OIDC_USERNAME_CLAIM":          "email",
				"OIDC_GROUPS_CLAIM":            "roles",
				"OIDC_ADMIN_GROUP":             "jellyfin-admins",
				"OIDC_ALLOWED_GROUP":           "jellyfin-users",
				"OIDC_LINK_EXISTING_USERS":     "true",
			},
			want: Config{
				DatabaseURL:        testDatabaseURL,
				HTTPPort:           9000,
				PublishedServerURL: "https://jellyfin.example",
				ServerID:           "0123456789abcdef0123456789abcdef",
				CORSOrigins:        []string{"https://one.example", "https://two.example"},
				Transcoder:         Transcoder{Jobs: 4, StallTimeout: 45 * time.Second},
				Temporal:           Temporal{HostPort: "temporal:7233", Namespace: "gojellyfin_production", WorkerStopTimeout: 5 * time.Minute},
				Tracing:            Tracing{OTLPEndpoint: "http://collector:4318"},
				TMDB:               TMDB{APIKey: "not-a-real-key"},
				MediaDirectories:   []string{"/media", "/library"},
				OIDC: OIDC{
					IssuerURL:         "https://auth.example",
					ClientID:          "gojellyfin",
					ClientSecret:      "not-a-real-secret",
					Scopes:            "openid email",
					UsernameClaim:     "email",
					GroupsClaim:       "roles",
					AdminGroup:        "jellyfin-admins",
					AllowedGroup:      "jellyfin-users",
					LinkExistingUsers: true,
				},
				SourceAPIKeys: map[string]string{},
			},
		},
		{
			name: "collects only the prefixed source api keys",
			env: map[string]string{
				"DATABASE_URL":             testDatabaseURL,
				"SOURCE_API_KEY_RADARR":    "radarr-key",
				"SOURCE_API_KEY_SONARR_4K": "sonarr-key",
				"SOURCE_API_KEY_":          "no-name",
				"SOURCE_API_KEY_EMPTY":     "",
				"TMDB_API_KEY":             "not-a-real-key",
				"RADARR_API_KEY":           "unprefixed",
			},
			want: Config{
				DatabaseURL:      testDatabaseURL,
				HTTPPort:         defaultHTTPPort,
				CORSOrigins:      []string{},
				MediaDirectories: []string{"/"},
				TMDB:             TMDB{APIKey: "not-a-real-key"},
				SourceAPIKeys: map[string]string{
					"SOURCE_API_KEY_RADARR":    "radarr-key",
					"SOURCE_API_KEY_SONARR_4K": "sonarr-key",
				},
			},
		},
		{
			name:    "refuses an unset DATABASE_URL",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name:    "refuses a non numeric TRANSCODER_JOBS",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "TRANSCODER_JOBS": "lots"},
			wantErr: true,
		},
		{
			name:    "refuses a negative TRANSCODER_JOBS",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "TRANSCODER_JOBS": "-2"},
			wantErr: true,
		},
		{
			name:    "refuses a unitless TRANSCODER_STALL_TIMEOUT",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "TRANSCODER_STALL_TIMEOUT": "30"},
			wantErr: true,
		},
		{
			name:    "refuses a unitless TEMPORAL_WORKER_STOP_TIMEOUT",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "TEMPORAL_WORKER_STOP_TIMEOUT": "300"},
			wantErr: true,
		},
		{
			name:    "refuses a negative TEMPORAL_WORKER_STOP_TIMEOUT",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "TEMPORAL_WORKER_STOP_TIMEOUT": "-1m"},
			wantErr: true,
		},
		{
			name:    "refuses a SERVER_ID that is not what device.txt holds",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "SERVER_ID": "E10A32FC-A793-42D7-B8B9-D96E255CE1BC"},
			wantErr: true,
		},
		{
			name:    "refuses a non numeric HTTP_PORT",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "HTTP_PORT": "http"},
			wantErr: true,
		},
		{
			name:    "refuses an OIDC_ISSUER_URL that is not a URL",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "PUBLISHED_SERVER_URL": "https://jellyfin.example", "OIDC_ISSUER_URL": "auth.example", "OIDC_CLIENT_ID": "gojellyfin"},
			wantErr: true,
		},
		{
			name:    "refuses an OIDC_ISSUER_URL without OIDC_CLIENT_ID",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "PUBLISHED_SERVER_URL": "https://jellyfin.example", "OIDC_ISSUER_URL": "https://auth.example"},
			wantErr: true,
		},
		{
			name:    "refuses an OIDC_ISSUER_URL without PUBLISHED_SERVER_URL",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "OIDC_ISSUER_URL": "https://auth.example", "OIDC_CLIENT_ID": "gojellyfin"},
			wantErr: true,
		},
		{
			name:    "refuses a non boolean OIDC_LINK_EXISTING_USERS",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "OIDC_LINK_EXISTING_USERS": "sure"},
			wantErr: true,
		},
		{
			name:    "refuses an HTTP_PORT above 65535",
			env:     map[string]string{"DATABASE_URL": testDatabaseURL, "HTTP_PORT": "70000"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setEnvironment(t, test.env)

			config, err := Load()

			if test.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, config)
		})
	}
}
