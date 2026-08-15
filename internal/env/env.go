package env

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultHTTPPort = 8081
	maxPort         = 65535
)

// Config is everything this process reads from its environment, read once at
// start. Nothing else reads it, so the knobs are discoverable by reading one
// struct rather than by grepping, and a package under test is handed a value
// instead of having to set a variable.
type Config struct {
	DatabaseURL        string     `mapstructure:"DATABASE_URL"`
	HTTPPort           int        `mapstructure:"HTTP_PORT"`
	PublishedServerURL string     `mapstructure:"PUBLISHED_SERVER_URL"` // what the public endpoints declare the server on
	CORSOrigins        []string   `mapstructure:"CORS_ORIGINS"`
	Transcoder         Transcoder `mapstructure:",squash"`
	Temporal           Temporal   `mapstructure:",squash"`
}

type Transcoder struct {
	Jobs         int           `mapstructure:"TRANSCODER_JOBS"`
	StallTimeout time.Duration `mapstructure:"TRANSCODER_STALL_TIMEOUT"`
}

type Temporal struct {
	HostPort  string `mapstructure:"TEMPORAL_HOSTPORT"`
	Namespace string `mapstructure:"TEMPORAL_NAMESPACE"`
}

func Load() (Config, error) {
	v := viper.New()
	v.SetDefault("HTTP_PORT", defaultHTTPPort)
	v.SetEnvPrefix("")
	v.AutomaticEnv()

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return Config{}, fmt.Errorf("the environment could not be read: %w", err)
	}

	config.CORSOrigins = trimmed(config.CORSOrigins)

	if err := config.validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is not set, so there is no database to open")
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

	return nil
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
