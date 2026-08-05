package middleware

import "net/http"

type HttpMiddleware func(next http.Handler) http.Handler
