package env

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultTemporalNamespace = "gojellyfin_default"
	defaultHTTPPort          = 8081
	maxPort                  = 65535
)

// Config is everything this process reads from its environment, read once at
// start. Nothing else reads it, so the knobs are discoverable by reading one
// struct rather than by grepping, and a package under test is handed a value
// instead of having to set a variable.
type Config struct {
	// No default: a process that was not told which database to open should say
	// so rather than quietly dial localhost. The Makefile sets it for dev.
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	HTTPPort int `mapstructure:"HTTP_PORT"`

	// Empty advertises no address at all. A client prefers LocalAddress when it
	// believes it shares a network with the server, so naming one this server
	// cannot confirm sends the client somewhere it cannot reach.
	PublishedServerURL string `mapstructure:"PUBLISHED_SERVER_URL"`

	// Empty reflects whatever origin asks. A literal "*" is not equivalent:
	// browsers reject it on credentialed requests.
	CORSOrigins []string `mapstructure:"CORS_ORIGINS"`

	Transcoder Transcoder `mapstructure:",squash"`

	Temporal Temporal `mapstructure:",squash"`
}

type Transcoder struct {
	// Zero means the core count, because one encode saturates about a core.
	Jobs         int           `mapstructure:"TRANSCODER_JOBS"`
	StallTimeout time.Duration `mapstructure:"TRANSCODER_STALL_TIMEOUT"`
}

type Temporal struct {
	// Empty leaves background work off rather than failing to start, so a
	// developer running the server alone gets a server, not a dial error.
	HostPort  string `mapstructure:"TEMPORAL_HOSTPORT"`
	Namespace string `mapstructure:"TEMPORAL_NAMESPACE"`
}

var keys = []string{
	"DATABASE_URL",
	"HTTP_PORT",
	"PUBLISHED_SERVER_URL",
	"CORS_ORIGINS",
	"TRANSCODER_JOBS",
	"TRANSCODER_STALL_TIMEOUT",
	"TEMPORAL_HOSTPORT",
	"TEMPORAL_NAMESPACE",
}

func Load() (Config, error) {
	v := viper.New()
	v.SetDefault("HTTP_PORT", defaultHTTPPort)
	v.SetDefault("TEMPORAL_NAMESPACE", defaultTemporalNamespace)

	for _, key := range keys {
		if err := v.BindEnv(key); err != nil {
			return Config{}, err
		}
	}

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

// A malformed value is refused rather than ignored: silently falling back to
// the default is how a typo in a manifest becomes a capacity problem nobody
// can explain.
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

// MustLoad is for tests and one-shot tools, where a bad environment should stop
// the process rather than be threaded back to a caller that cannot act on it.
func MustLoad() Config {
	config, err := Load()
	if err != nil {
		panic(err)
	}

	return config
}
