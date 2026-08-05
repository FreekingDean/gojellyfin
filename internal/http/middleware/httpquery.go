package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

// Jellyfin clients send query parameters in PascalCase, but the generated
// binding matches the spec's camelCase spelling exactly.
func HttpCanonicalQuery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			if canonical, ok := canonicalQuery(r.URL.Query()); ok {
				r.URL.RawQuery = canonical.Encode()
			}
		}

		next.ServeHTTP(w, r)
	})
}

func canonicalQuery(values url.Values) (url.Values, bool) {
	canonical := make(url.Values, len(values))
	changed := false

	for key, value := range values {
		if name, ok := api.QueryParameters[strings.ToLower(key)]; ok && name != key {
			key = name
			changed = true
		}
		canonical[key] = append(canonical[key], value...)
	}

	return canonical, changed
}
