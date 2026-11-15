package env

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultHTTPPort        = 8081
	defaultDatabaseConns   = 20
	defaultConnMaxLifetime = 30 * time.Minute
	maxPort                = 65535

	SourceAPIKeyPrefix = "SOURCE_API_KEY_"
)

var serverIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Config struct {
	DatabaseURL        string     `mapstructure:"DATABASE_URL" secret:"true"`
	Database           Database   `mapstructure:",squash"`
	HTTPPort           int        `mapstructure:"HTTP_PORT"`
	PublishedServerURL string     `mapstructure:"PUBLISHED_SERVER_URL"`
	ServerID           string     `mapstructure:"SERVER_ID"`
	CORSOrigins        []string   `mapstructure:"CORS_ORIGINS"`
	TrustedProxies     []string   `mapstructure:"TRUSTED_PROXIES"`
	Transcoder         Transcoder `mapstructure:",squash"`
	Temporal           Temporal   `mapstructure:",squash"`
	Tracing            Tracing    `mapstructure:",squash"`
	TMDB               TMDB       `mapstructure:",squash"`
	OIDC               OIDC       `mapstructure:",squash"`
	MediaDirectories   []string   `mapstructure:"MEDIA_DIRECTORIES"`

	SourceAPIKeys map[string]string `mapstructure:"-"`
}

type Database struct {
	MaxConns        int           `mapstructure:"DATABASE_MAX_CONNS"`
	ConnMaxLifetime time.Duration `mapstructure:"DATABASE_CONN_MAX_LIFETIME"`
}

type Transcoder struct {
	Jobs         int           `mapstructure:"TRANSCODER_JOBS"`
	StallTimeout time.Duration `mapstructure:"TRANSCODER_STALL_TIMEOUT"`
}

type Temporal struct {
	HostPort          string        `mapstructure:"TEMPORAL_HOSTPORT"`
	Namespace         string        `mapstructure:"TEMPORAL_NAMESPACE"`
	WorkerStopTimeout time.Duration `mapstructure:"TEMPORAL_WORKER_STOP_TIMEOUT"`
}

type Tracing struct {
	OTLPEndpoint       string `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	ServiceName        string `mapstructure:"OTEL_SERVICE_NAME"`
	ResourceAttributes string `mapstructure:"OTEL_RESOURCE_ATTRIBUTES"`
}

type TMDB struct {
	APIKey  string `mapstructure:"TMDB_API_KEY" secret:"true"`
	BaseURL string `mapstructure:"TMDB_BASE_URL"`
}

type OIDC struct {
	IssuerURL         string `mapstructure:"OIDC_ISSUER_URL"`
	ClientID          string `mapstructure:"OIDC_CLIENT_ID"`
	ClientSecret      string `mapstructure:"OIDC_CLIENT_SECRET" secret:"true"`
	Scopes            string `mapstructure:"OIDC_SCOPES"`
	UsernameClaim     string `mapstructure:"OIDC_USERNAME_CLAIM"`
	GroupsClaim       string `mapstructure:"OIDC_GROUPS_CLAIM"`
	AdminGroup        string `mapstructure:"OIDC_ADMIN_GROUP"`
	AllowedGroup      string `mapstructure:"OIDC_ALLOWED_GROUP"`
	LinkExistingUsers bool   `mapstructure:"OIDC_LINK_EXISTING_USERS"`
}

func Load() (Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetDefault("HTTP_PORT", defaultHTTPPort)
	v.SetDefault("DATABASE_MAX_CONNS", defaultDatabaseConns)
	v.SetDefault("DATABASE_CONN_MAX_LIFETIME", defaultConnMaxLifetime)
	v.SetDefault("MEDIA_DIRECTORIES", []string{"/"})

	v.AutomaticEnv()

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return Config{}, fmt.Errorf("the environment could not be read: %w", err)
	}

	config.CORSOrigins = trimmed(config.CORSOrigins)
	config.TrustedProxies = trimmed(config.TrustedProxies)
	config.MediaDirectories = trimmed(config.MediaDirectories)
	config.SourceAPIKeys = sourceAPIKeys()

	if err := config.validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is not set, so there is no database to open")
	}
	if c.Database.MaxConns < 1 {
		return fmt.Errorf("DATABASE_MAX_CONNS must be a whole number of connections above zero, got %d", c.Database.MaxConns)
	}
	if c.Database.ConnMaxLifetime <= 0 {
		return fmt.Errorf("DATABASE_CONN_MAX_LIFETIME must be a duration above zero such as 30m, got %s", c.Database.ConnMaxLifetime)
	}
	if c.HTTPPort < 1 || c.HTTPPort > maxPort {
		return fmt.Errorf("HTTP_PORT must be a port between 1 and %d, got %d", maxPort, c.HTTPPort)
	}
	if c.Transcoder.Jobs < 0 {
		return fmt.Errorf("TRANSCODER_JOBS must be a positive whole number, got %d", c.Transcoder.Jobs)
	}
	if c.Transcoder.StallTimeout < 0 {
		return fmt.Errorf("TRANSCODER_STALL_TIMEOUT must be a positive duration such as 30s, got %s", c.Transcoder.StallTimeout)
	}
	if c.Temporal.WorkerStopTimeout < 0 {
		return fmt.Errorf("TEMPORAL_WORKER_STOP_TIMEOUT must be a positive duration such as 5m, got %s", c.Temporal.WorkerStopTimeout)
	}
	if c.ServerID != "" && !serverIDPattern.MatchString(c.ServerID) {
		return fmt.Errorf("SERVER_ID must be 32 lowercase hex characters, the form Jellyfin writes to device.txt, got %q", c.ServerID)
	}

	for _, proxy := range c.TrustedProxies {
		if _, err := netip.ParsePrefix(proxy); err != nil {
			return fmt.Errorf("TRUSTED_PROXIES must be a comma separated list of networks such as 10.201.0.0/16, got %q", proxy)
		}
	}

	return c.validateOIDC()
}

func (c Config) validateOIDC() error {
	if c.OIDC.IssuerURL == "" {
		return nil
	}
	if issuer, err := url.Parse(c.OIDC.IssuerURL); err != nil || !issuer.IsAbs() || issuer.Host == "" {
		return fmt.Errorf("OIDC_ISSUER_URL must be an absolute URL such as https://auth.example, got %q", c.OIDC.IssuerURL)
	}
	if c.OIDC.ClientID == "" {
		return errors.New("OIDC_ISSUER_URL is set but OIDC_CLIENT_ID is not, and a provider cannot be asked to sign in an unnamed client")
	}
	if published, err := url.Parse(c.PublishedServerURL); c.PublishedServerURL == "" || err != nil || !published.IsAbs() || published.Host == "" {
		return errors.New("OIDC_ISSUER_URL is set but PUBLISHED_SERVER_URL is not an absolute URL, and the provider needs one to send the browser back to")
	}

	return nil
}

func sourceAPIKeys() map[string]string {
	found := map[string]string{}
	for _, entry := range os.Environ() {
		name, value, split := strings.Cut(entry, "=")
		if !split || value == "" || !ValidSourceAPIKeyVariable(name) {
			continue
		}
		found[name] = value
	}

	return found
}

func ValidSourceAPIKeyVariable(name string) bool {
	return strings.HasPrefix(name, SourceAPIKeyPrefix) && len(name) > len(SourceAPIKeyPrefix)
}

func trimmed(values []string) []string {
	entries := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			entries = append(entries, value)
		}
	}

	return entries
}
