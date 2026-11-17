package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	"github.com/FreekingDean/gojellyfin/internal/server"
	"github.com/FreekingDean/gojellyfin/internal/server/admin"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/link"
	"github.com/FreekingDean/gojellyfin/internal/server/report"
	"github.com/FreekingDean/gojellyfin/internal/server/request"
	"github.com/FreekingDean/gojellyfin/internal/server/socket"
	"github.com/FreekingDean/gojellyfin/internal/server/sso"
	"github.com/FreekingDean/gojellyfin/internal/server/stream"
	"github.com/FreekingDean/gojellyfin/internal/server/webhook"
)

type streamRoute struct {
	pattern   string
	operation string
	handler   func(*stream.Handler) http.HandlerFunc
}

func serve(h *stream.Handler) http.HandlerFunc          { return h.Serve }
func serveUniversal(h *stream.Handler) http.HandlerFunc { return h.ServeUniversal }
func serveMaster(h *stream.Handler) http.HandlerFunc    { return h.ServeMasterPlaylist }
func serveMedia(h *stream.Handler) http.HandlerFunc     { return h.ServeMediaPlaylist }
func serveAudio(h *stream.Handler) http.HandlerFunc     { return h.ServeAudioPlaylist }
func serveSegment(h *stream.Handler) http.HandlerFunc   { return h.ServeSegment }

var streamRoutes = []streamRoute{
	{"GET /Videos/{itemId}/stream", "GetVideoStream", serve},
	{"HEAD /Videos/{itemId}/stream", "HeadVideoStream", serve},
	{"GET /Videos/{itemId}/stream.{container}", "GetVideoStreamByContainer", serve},
	{"HEAD /Videos/{itemId}/stream.{container}", "HeadVideoStreamByContainer", serve},
	{"GET /Audio/{itemId}/stream", "GetAudioStream", serve},
	{"HEAD /Audio/{itemId}/stream", "HeadAudioStream", serve},
	{"GET /Audio/{itemId}/stream.{container}", "GetAudioStreamByContainer", serve},
	{"HEAD /Audio/{itemId}/stream.{container}", "HeadAudioStreamByContainer", serve},
	{"GET /Videos/{itemId}/master.m3u8", "GetMasterHlsVideoPlaylist", serveMaster},
	{"HEAD /Videos/{itemId}/master.m3u8", "HeadMasterHlsVideoPlaylist", serveMaster},
	{"GET /Videos/{itemId}/live.m3u8", "GetLiveHlsStream", serveMaster},
	{"GET /Videos/{itemId}/main.m3u8", "GetVariantHlsVideoPlaylist", serveMedia},
	{"GET /Videos/{itemId}/audio.m3u8", "GetVariantHlsAudioPlaylist", serveAudio},
	{"GET /Videos/{itemId}/hls1/{playlistId}/{segmentId}.{container}", "GetHlsVideoSegment", serveSegment},
	{"GET /Audio/{itemId}/universal", "GetUniversalAudioStream", serveUniversal},
	{"HEAD /Audio/{itemId}/universal", "HeadUniversalAudioStream", serveUniversal},
}

var legacyRoutes = map[string]string{
	"GET /Users/{userId}/Items":                            "/Items",
	"GET /Users/{userId}/Items/Latest":                     "/Items/Latest",
	"GET /Users/{userId}/Items/Resume":                     "/UserItems/Resume",
	"GET /Users/{userId}/Items/Root":                       "/Items/Root",
	"GET /Users/{userId}/Items/{itemId}":                   "/Items/{itemId}",
	"GET /Users/{userId}/Items/{itemId}/Intros":            "/Items/{itemId}/Intros",
	"GET /Users/{userId}/Items/{itemId}/LocalTrailers":     "/Items/{itemId}/LocalTrailers",
	"GET /Users/{userId}/Items/{itemId}/SpecialFeatures":   "/Items/{itemId}/SpecialFeatures",
	"GET /Users/{userId}/Items/{itemId}/UserData":          "/UserItems/{itemId}/UserData",
	"POST /Users/{userId}/Items/{itemId}/UserData":         "/UserItems/{itemId}/UserData",
	"POST /Users/{userId}/Items/{itemId}/Rating":           "/UserItems/{itemId}/Rating",
	"DELETE /Users/{userId}/Items/{itemId}/Rating":         "/UserItems/{itemId}/Rating",
	"POST /Users/{userId}/FavoriteItems/{itemId}":          "/UserFavoriteItems/{itemId}",
	"DELETE /Users/{userId}/FavoriteItems/{itemId}":        "/UserFavoriteItems/{itemId}",
	"POST /Users/{userId}/PlayedItems/{itemId}":            "/UserPlayedItems/{itemId}",
	"DELETE /Users/{userId}/PlayedItems/{itemId}":          "/UserPlayedItems/{itemId}",
	"POST /Users/{userId}/PlayingItems/{itemId}":           "/PlayingItems/{itemId}",
	"DELETE /Users/{userId}/PlayingItems/{itemId}":         "/PlayingItems/{itemId}",
	"POST /Users/{userId}/PlayingItems/{itemId}/Progress":  "/PlayingItems/{itemId}/Progress",
	"GET /Users/{userId}/Views":                            "/UserViews",
	"GET /MusicGenres":                                     "/Genres",
	"GET /Users/{userId}/GroupingOptions":                  "/UserViews/GroupingOptions",
	"GET /Users/{userId}/Suggestions":                      "/Items/Suggestions",
	"GET /Users/{userId}/Images/{imageType}":               "/UserImage",
	"HEAD /Users/{userId}/Images/{imageType}":              "/UserImage",
	"POST /Users/{userId}/Images/{imageType}":              "/UserImage",
	"DELETE /Users/{userId}/Images/{imageType}":            "/UserImage",
	"GET /Users/{userId}/Images/{imageType}/{imageIndex}":  "/UserImage",
	"HEAD /Users/{userId}/Images/{imageType}/{imageIndex}": "/UserImage",
	"POST /Users/{userId}/Images/{imageType}/{index}":      "/UserImage",
	"DELETE /Users/{userId}/Images/{imageType}/{index}":    "/UserImage",
	"POST /Users/{userId}/Configuration":                   "/Users/Configuration",
	"POST /Users/{userId}/Password":                        "/Users/Password",
	"POST /Users/{userId}":                                 "/Users",
}

var legacyPlaybackRoutes = map[string]string{
	"POST /PlayingItems/{itemId}":          "/Sessions/Playing",
	"POST /PlayingItems/{itemId}/Progress": "/Sessions/Playing/Progress",
	"DELETE /PlayingItems/{itemId}":        "/Sessions/Playing/Stopped",
}

type playbackKind int

const (
	playbackText playbackKind = iota
	playbackNumber
	playbackFlag
)

type playbackField struct {
	name string
	kind playbackKind
}

var playbackFields = map[string]playbackField{
	"mediasourceid":       {"MediaSourceId", playbackText},
	"playsessionid":       {"PlaySessionId", playbackText},
	"livestreamid":        {"LiveStreamId", playbackText},
	"playmethod":          {"PlayMethod", playbackText},
	"repeatmode":          {"RepeatMode", playbackText},
	"nextmediatype":       {"NextMediaType", playbackText},
	"positionticks":       {"PositionTicks", playbackNumber},
	"audiostreamindex":    {"AudioStreamIndex", playbackNumber},
	"subtitlestreamindex": {"SubtitleStreamIndex", playbackNumber},
	"volumelevel":         {"VolumeLevel", playbackNumber},
	"canseek":             {"CanSeek", playbackFlag},
	"ispaused":            {"IsPaused", playbackFlag},
	"ismuted":             {"IsMuted", playbackFlag},
}

func legacyPlaybackRoute(m *mux.Mux, target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := map[string]any{"ItemId": r.PathValue("itemId")}
		for key, values := range r.URL.Query() {
			field, ok := playbackFields[strings.ToLower(key)]
			if !ok || len(values) == 0 || values[0] == "" {
				continue
			}

			value, err := playbackValue(field.kind, values[0])
			if err != nil {
				http.Error(w, fmt.Sprintf("%s: %v", key, err), http.StatusBadRequest)
				return
			}
			report[field.name] = value
		}

		body, err := json.Marshal(report)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		r.Method = http.MethodPost
		r.URL.Path = target
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		r.Header.Set("Content-Type", "application/json")

		m.ServeHTTP(w, r)
	}
}

func playbackValue(kind playbackKind, value string) (any, error) {
	switch kind {
	case playbackNumber:
		return strconv.ParseInt(value, 10, 64)
	case playbackFlag:
		return strconv.ParseBool(value)
	default:
		return value, nil
	}
}

var routeParam = regexp.MustCompile(`\{([^}]+)\}`)

func legacyPatterns() []string {
	patterns := make([]string, 0, len(legacyRoutes))
	for pattern := range legacyRoutes {
		patterns = append(patterns, pattern)
	}

	slices.SortFunc(patterns, func(a, b string) int {
		if byLiteral := literalSegments(b) - literalSegments(a); byLiteral != 0 {
			return byLiteral
		}

		return strings.Compare(a, b)
	})

	return patterns
}

func literalSegments(pattern string) int {
	count := 0
	for _, segment := range strings.Split(pattern, "/") {
		if !strings.Contains(segment, "{") {
			count++
		}
	}

	return count
}

func legacyRoute(m *mux.Mux, target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		query.Set("userId", r.PathValue("userId"))

		r.URL.Path = routeParam.ReplaceAllStringFunc(target, func(param string) string {
			return r.PathValue(strings.Trim(param, "{}"))
		})
		r.URL.RawQuery = query.Encode()

		m.ServeHTTP(w, r)
	}
}

type Server struct {
	s *http.Server

	tracing        *tracing.Tracing
	httpMiddleware []middleware.HttpMiddleware
	apiMiddleware  []api.StrictMiddlewareFunc
	apiOptions     api.StrictHTTPServerOptions
}

const castReceiverOrigin = "https://apps.jellyfin.org"

func newHTTPMiddleware(config env.Config) []middleware.HttpMiddleware {
	origins := append([]string{castReceiverOrigin}, config.CORSOrigins...)

	return []middleware.HttpMiddleware{middleware.HttpCORS(origins), middleware.HttpClientAddress(middleware.Prefixes(config.TrustedProxies)), middleware.HttpLogging, middleware.HttpCanonicalQuery}
}

func newAPIMiddleware(tracing *tracing.Tracing, tracingMiddleware *middleware.OapiTracing, authMiddleware *middleware.Auth, policies middleware.Policies) []api.StrictMiddlewareFunc {
	stack := []api.StrictMiddlewareFunc{
		middleware.OapiLogging,
		middleware.Authorize(policies),
		authMiddleware.Middleware,
	}
	if tracing.Enabled() {
		stack = append(stack, tracingMiddleware.Middleware)
	}

	return stack
}

func New(config env.Config, m *mux.Mux, authMiddleware *middleware.Auth, tracing *tracing.Tracing, tracingMiddleware *middleware.OapiTracing, policies middleware.Policies) *Server {
	return &Server{
		s: &http.Server{
			Addr: fmt.Sprintf(":%d", config.HTTPPort),
		},

		tracing: tracing,

		httpMiddleware: newHTTPMiddleware(config),

		apiMiddleware: newAPIMiddleware(tracing, tracingMiddleware, authMiddleware, policies),

		apiOptions: api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("bad request: %s %s: %v", r.Method, r.RequestURI, err)
				http.Error(w, err.Error(), http.StatusBadRequest)
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("Error: %s %s: %v", r.Method, r.URL.Path, err)
				if errors.Is(err, api.ErrNotImplemented) {
					http.Error(w, err.Error(), http.StatusNotImplemented)
					return
				}
				if errors.Is(err, auth.ErrUnauthorized) {
					http.Error(w, err.Error(), http.StatusUnauthorized)
					return
				}
				if errors.Is(err, auth.ErrForbidden) {
					http.Error(w, err.Error(), http.StatusForbidden)
					return
				}
				if errors.Is(err, items.ErrNotFound) {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
			},
		},
	}
}

func Register(
	s *Server,
	apiServer *server.Server,
	sock *socket.Socket,
	links *link.Handler,
	administration *admin.Handler,
	reporting *report.Handler,
	requesting *request.Handler,
	hooks *webhook.Handler,
	streams *stream.Handler,
	signOn *sso.Handler,
	m *mux.Mux,
) {
	h := api.NewStrictHandlerWithOptions(apiServer, s.apiMiddleware, s.apiOptions)
	m.HandleFunc("GET /socket", sock.Handle)
	registerRoot(m)
	links.Register(m)
	administration.Register(m)
	reporting.Register(m)
	requesting.Register(m)
	hooks.Register(m)
	for _, pattern := range sso.Routes {
		m.HandleFunc(pattern, signOn.Serve)
	}

	for _, route := range streamRoutes {
		m.HandleFunc(route.pattern, middleware.StreamTracing(s.tracing, route.operation, streams.Tagged(route.handler(streams))))
	}
	m.HandleFunc("DELETE /Videos/ActiveEncodings", streams.StopEncodings)
	finalHandler := api.HandlerWithOptions(h, api.StdHTTPServerOptions{
		BaseRouter: m,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("bad request: %s %s: %v", r.Method, r.RequestURI, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
	})
	for _, pattern := range legacyPatterns() {
		m.HandleFunc(pattern, legacyRoute(m, legacyRoutes[pattern]))
	}
	for pattern, target := range legacyPlaybackRoutes {
		m.HandleFunc(pattern, legacyPlaybackRoute(m, target))
	}
	for _, mw := range s.httpMiddleware {
		finalHandler = mw(finalHandler)
	}
	s.s.Handler = finalHandler
}

func (s *Server) ListenAndServe() {
	if err := s.s.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Printf("ListenAndServe error: %v", err)
		}
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	if err := s.s.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}
