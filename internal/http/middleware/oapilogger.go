package middleware

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func OapiLogging(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		start := time.Now()
		resp, err := f(ctx, w, r, request)
		if trace := tracing.TraceID(ctx); trace != "" {
			log.Printf("%s %s %s %s %v trace=%s", r.Method, loggedURI(r), operationID, time.Since(start), err, trace)
		} else {
			log.Printf("%s %s %s %s %v", r.Method, loggedURI(r), operationID, time.Since(start), err)
		}
		return resp, err
	}
}
