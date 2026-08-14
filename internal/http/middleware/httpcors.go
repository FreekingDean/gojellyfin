package middleware

import (
	"net/http"
	"strings"

	"github.com/rs/cors"
)

func HttpCORS(origins []string) HttpMiddleware {
	return func(next http.Handler) http.Handler {
		c := cors.New(cors.Options{
			AllowOriginFunc:  allowOrigin(origins),
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
		})

		return c.Handler(next)
	}
}

// Clients are web apps on origins this server cannot know, so an unset list
// reflects whatever origin asks. A literal "*" is not the same thing: a browser
// rejects it on any credentialed request, which is every authenticated call.
func allowOrigin(allowed []string) func(string) bool {
	origins := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		origins[strings.ToLower(strings.TrimSuffix(origin, "/"))] = struct{}{}
	}

	if len(origins) == 0 {
		return func(string) bool { return true }
	}

	return func(origin string) bool {
		_, ok := origins[strings.ToLower(strings.TrimSuffix(origin, "/"))]

		return ok
	}
}
