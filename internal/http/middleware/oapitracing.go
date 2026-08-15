package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type OapiTracing struct {
	tracing *tracing.Tracing
}

func NewOapiTracing(t *tracing.Tracing) *OapiTracing {
	return &OapiTracing{tracing: t}
}

func (t *OapiTracing) Middleware(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		if streams(r.URL.Path) {
			return f(ctx, w, r, request)
		}

		ctx, span := t.tracing.StartRequest(ctx, r.Header, operationID, r.Method)
		defer span.End()

		resp, err := f(ctx, w, r, request)
		if err != nil {
			span.Fail(err)
		}

		return resp, err
	}
}

// Everything under these two roots is a progressive response that runs for the
// length of the media, so a span covering one is open for hours. They are the
// same roots deploy/httproutes.yaml sends to the transcode pods. The mux
// matches case-insensitively, so /videos has to be excluded like /Videos.
func streams(path string) bool {
	root, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")

	return slices.ContainsFunc([]string{"Videos", "Audio"}, func(untraced string) bool {
		return strings.EqualFold(untraced, root)
	})
}
