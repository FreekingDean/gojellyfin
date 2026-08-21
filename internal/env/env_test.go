package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testDatabaseURL = "postgres://localhost:5432/test?sslmode=disable"

func TestLoad(t *testing.T) {
	tests := []struct {
		name          string
		envs          map[string]string
		assertionfunc func(*testing.T, Config, error)
	}{
		{
			"http_port is not set",
			map[string]string{"HTTP_PORT": ""},
			func(t *testing.T, c Config, err error) {
				assert.Equal(t, 8081, c.HTTPPort)
				assert.NoError(t, err)
			},
		},
		{
			"database_url is not set",
			map[string]string{"DATABASE_URL": ""},
			func(t *testing.T, c Config, err error) {
				assert.Error(t, err, "hi")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originals := map[string]string{}
			for name := range test.envs {
				originals[name] = os.Getenv(name)
			}
			os.Setenv("DATABASE_URL", "fake-url")
			for name, val := range test.envs {
				os.Setenv(name, val)
			}

			config, err := Load()

			test.assertionfunc(t, config, err)

			for name := range test.envs {
				os.Setenv(name, originals[name])
			}
		})
	}
}
