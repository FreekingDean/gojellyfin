package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

func HttpCORS(origins []string) HttpMiddleware {
	return func(next http.Handler) http.Handler {
		c := cors.New(cors.Options{
			AllowedOrigins:   origins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
		})

		return c.Handler(next)
	}
}
