package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

// Two things the generated binding cannot cope with on its own: Jellyfin
// clients send query parameters in PascalCase while the spec declares them
// camelCase, and they send `Param=` with an empty value where a value is
// undefined, which fails to parse as an int or a uuid and answers 400. Upstream
// treats empty as absent, so it is dropped here.
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

		for _, single := range value {
			if single == "" {
				changed = true
				continue
			}
			canonical[key] = append(canonical[key], single)
		}
	}

	return canonical, changed
}
