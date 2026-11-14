package env

import (
	"slices"
	"testing"
	"time"
)

func TestConfig_Variables(t *testing.T) {
	config := Config{
		DatabaseURL:      "postgres://user:hunter2@db/gojellyfin",
		HTTPPort:         8081,
		MediaDirectories: []string{"/media", "/archive"},
		CORSOrigins:      []string{},
		Transcoder:       Transcoder{StallTimeout: 30 * time.Second},
		TMDB:             TMDB{APIKey: "tmdb-key"},
		OIDC:             OIDC{ClientSecret: "oidc-secret", LinkExistingUsers: true},
		SourceAPIKeys:    map[string]string{"SOURCE_API_KEY_RADARR": "radarr-key", "SOURCE_API_KEY_ALPHA": "alpha-key"},
	}

	variables := config.Variables()
	found := map[string]Variable{}
	names := make([]string, 0, len(variables))
	for _, variable := range variables {
		found[variable.Name] = variable
		names = append(names, variable.Name)
	}

	want := append(slices.Clone(settings), "SOURCE_API_KEY_ALPHA", "SOURCE_API_KEY_RADARR")
	slices.Sort(want)
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Errorf("variables = %v, want %v", names, want)
	}

	for _, name := range []string{"DATABASE_URL", "TMDB_API_KEY", "OIDC_CLIENT_SECRET", "SOURCE_API_KEY_RADARR", "SOURCE_API_KEY_ALPHA"} {
		if got, want := found[name], (Variable{Name: name, Set: true, Secret: true}); got != want {
			t.Errorf("%s = %+v, want %+v", name, got, want)
		}
	}

	tests := []Variable{
		{Name: "HTTP_PORT", Value: "8081", Set: true},
		{Name: "MEDIA_DIRECTORIES", Value: "/media, /archive", Set: true},
		{Name: "TRANSCODER_STALL_TIMEOUT", Value: "30s", Set: true},
		{Name: "OIDC_LINK_EXISTING_USERS", Value: "true", Set: true},
		{Name: "OIDC_ISSUER_URL"},
		{Name: "CORS_ORIGINS"},
		{Name: "TRANSCODER_JOBS"},
	}
	for _, want := range tests {
		if got := found[want.Name]; got != want {
			t.Errorf("%s = %+v, want %+v", want.Name, got, want)
		}
	}
}
