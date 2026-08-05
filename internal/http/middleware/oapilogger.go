package middleware

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func OapiLogging(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		start := time.Now()
		resp, err := f(ctx, w, r, request)
		log.Printf("%s %s %s %s %v", r.Method, r.RequestURI, operationID, time.Since(start), err)
		return resp, err
	}
}
