package middleware

import (
	"net/http"
	"strings"
)

var credentials = map[string]bool{"secret": true, "api_key": true, "apikey": true}

func loggedURI(r *http.Request) string {
	query := r.URL.Query()
	if len(query) == 0 {
		return r.URL.Path
	}

	for key := range query {
		if credentials[strings.ToLower(key)] {
			query.Set(key, "redacted")
		}
	}

	return r.URL.Path + "?" + query.Encode()
}
