package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultDatabaseURL = "postgres://localhost:5432/gojellyfin_development?sslmode=disable"

// Config is everything this process reads from its environment, read once at
// start. Nothing else calls os.Getenv, so the knobs are discoverable by reading
// one struct rather than by grepping, and a package under test is handed a
// value instead of having to set a variable.
type Config struct {
	DatabaseURL string

	// Empty advertises no address at all. A client prefers LocalAddress when it
	// believes it shares a network with the server, so naming one this server
	// cannot confirm sends the client somewhere it cannot reach.
	PublishedServerURL string

	// Empty reflects whatever origin asks. A literal "*" is not equivalent:
	// browsers reject it on credentialed requests.
	CORSOrigins []string

	Transcoder Transcoder
}

type Transcoder struct {
	// Zero means the core count, because one encode saturates about a core.
	Jobs         int
	StallTimeout time.Duration
}

func Load() (Config, error) {
	config := Config{
		DatabaseURL:        orElse(os.Getenv("DATABASE_URL"), defaultDatabaseURL),
		PublishedServerURL: os.Getenv("PUBLISHED_SERVER_URL"),
		CORSOrigins:        list(os.Getenv("CORS_ORIGINS")),
		Transcoder:         Transcoder{},
	}

	jobs, err := number("TRANSCODER_JOBS")
	if err != nil {
		return Config{}, err
	}
	config.Transcoder.Jobs = jobs

	stall, err := duration("TRANSCODER_STALL_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	config.Transcoder.StallTimeout = stall

	return config, nil
}

// A malformed number is refused rather than ignored: silently falling back to
// the default is how a typo in a manifest becomes a capacity problem nobody
// can explain.
func number(name string) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive whole number, got %q", name, raw)
	}

	return value, nil
}

func duration(name string) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as 30s, got %q", name, raw)
	}

	return value, nil
}

func list(value string) []string {
	entries := make([]string, 0)
	for _, entry := range strings.Split(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}

	return entries
}

func orElse(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
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
