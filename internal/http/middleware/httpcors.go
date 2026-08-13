package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/rs/cors"
)

func HttpCORS(next http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowOriginFunc:  allowOrigin(os.Getenv("CORS_ORIGINS")),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	return c.Handler(next)
}

// Clients are web apps on origins this server cannot know, so an unset list
// reflects whatever origin asks. A literal "*" is not the same thing: a browser
// rejects it on any credentialed request, which is every authenticated call.
func allowOrigin(value string) func(string) bool {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
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
