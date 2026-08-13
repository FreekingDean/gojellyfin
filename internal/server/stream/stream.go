package stream

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

var contentTypes = map[string]string{
	".mkv":  "video/x-matroska",
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".ts":   "video/mp2t",
	".mpg":  "video/mpeg",
	".mpeg": "video/mpeg",
	".mp3":  "audio/mpeg",
	".flac": "audio/flac",
	".m4a":  "audio/mp4",
	".m4b":  "audio/mp4",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/opus",
	".wav":  "audio/wav",
	".wma":  "audio/x-ms-wma",
}

// A transcode runs on a worker in another container, which hands back the
// output as one stream for the API to proxy.
type Transcoder interface {
	Enabled() bool
	Open(ctx context.Context, spec transcode.Spec) (io.ReadCloser, error)
}

type Handler struct {
	sessions   *sessions.Service
	items      *items.Service
	transcoder Transcoder
}

func New(sessions *sessions.Service, items *items.Service, transcoder Transcoder) *Handler {
	return &Handler{sessions: sessions, items: items, transcoder: transcoder}
}

// Registered ahead of the generated routes because the generated response type
// always writes a complete 200, which would break seeking.
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	item, ok := h.item(w, r)
	if !ok {
		return
	}

	container := sourceContainer(item)
	if requested := r.PathValue("container"); requested != "" && !isStatic(r) && !strings.EqualFold(requested, container) {
		if h.serveTranscode(w, r, item, []string{requested}) {
			return
		}

		unsupported(w, r, container, requested)
		return
	}

	h.serveFile(w, r, item)
}

func (h *Handler) ServeUniversal(w http.ResponseWriter, r *http.Request) {
	item, ok := h.item(w, r)
	if !ok {
		return
	}

	requested := r.URL.Query()["container"]
	profiles := directPlayProfiles(requested)
	if len(profiles) == 0 {
		h.serveFile(w, r, item)
		return
	}

	codec, err := h.items.AudioCodec(r.Context(), item.ID)
	if err != nil {
		log.Printf("failed to read the audio codec of %s: %v", item.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	container := sourceContainer(item)
	if !playable(profiles, container, codec) {
		containers := make([]string, 0, len(profiles))
		for _, profile := range profiles {
			containers = append(containers, profile.container)
		}
		if h.serveTranscode(w, r, item, containers) {
			return
		}

		unsupported(w, r, describe(container, codec), strings.Join(requested, ","))
		return
	}

	h.serveFile(w, r, item)
}

// Reports whether the response was answered by a transcode. Everything that
// can fail is done before the first byte reaches the client, so the caller can
// still refuse with a status when this comes back false.
func (h *Handler) serveTranscode(w http.ResponseWriter, r *http.Request, item *items.Item, accepted []string) bool {
	if !h.transcoder.Enabled() || !items.IsAudio(item) {
		return false
	}

	container := transcode.Choose(candidates(r, accepted))
	if container == "" {
		return false
	}

	output, err := h.transcoder.Open(r.Context(), transcode.Spec{
		Path:       item.Path,
		Container:  container,
		Bitrate:    audioBitrate(r),
		StartTicks: startTicks(r),
	})
	if err != nil {
		log.Printf("failed to transcode %s to %s: %v", item.Path, container, err)

		return false
	}
	defer func() {
		if err := output.Close(); err != nil {
			log.Printf("transcode of %s ended: %v", item.Path, err)
		}
	}()

	w.Header().Set("Content-Type", transcode.ContentType(container))
	// The length is not known until the encode finishes, so there is nothing
	// for a client to seek against.
	w.Header().Set("Accept-Ranges", "none")
	w.WriteHeader(http.StatusOK)

	if _, err := transcode.Relay(w, output); err != nil {
		log.Printf("transcode of %s stopped: %v", item.Path, err)
	}

	return true
}

// What the client asked to be transcoded to comes first, then the containers
// it said it can play, which it can equally take over a plain response.
func candidates(r *http.Request, accepted []string) []string {
	query := r.URL.Query()

	requested := query.Get("transcodingContainer")
	if requested == "" || strings.EqualFold(query.Get("transcodingProtocol"), "hls") {
		return accepted
	}

	return append([]string{requested}, accepted...)
}

func audioBitrate(r *http.Request) int32 {
	query := r.URL.Query()
	for _, name := range []string{"audioBitRate", "maxStreamingBitrate"} {
		if bitrate, err := strconv.ParseInt(query.Get(name), 10, 32); err == nil && bitrate > 0 {
			return int32(bitrate)
		}
	}

	return 0
}

func startTicks(r *http.Request) int64 {
	ticks, err := strconv.ParseInt(r.URL.Query().Get("startTimeTicks"), 10, 64)
	if err != nil || ticks < 0 {
		return 0
	}

	return ticks
}

// A client that cannot take the source as it is gets the reason rather than the
// wrong bytes, because there is no encoder to fall back on.
func unsupported(w http.ResponseWriter, r *http.Request, source, requested string) {
	log.Printf("no direct play for %s: the source is %s, the client accepts %s", r.URL.Path, source, requested)
	http.Error(w, "no direct play: the source is "+source, http.StatusUnsupportedMediaType)
}

func (h *Handler) item(w http.ResponseWriter, r *http.Request) (*items.Item, bool) {
	token := middleware.TokenFrom(r)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, false
	}
	if _, err := h.sessions.ByToken(r.Context(), token); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, false
	}

	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, false
	}

	item, err := h.items.ItemByID(r.Context(), id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return nil, false
	}

	return item, true
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, item *items.Item) {
	file, err := os.Open(item.Path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if contentType, ok := contentTypes[strings.ToLower(filepath.Ext(item.Path))]; ok {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeContent(w, r, filepath.Base(item.Path), info.ModTime(), file)
}

// Clients declare their playable formats the way upstream's device profile
// parses them: a comma separated list of "container", each container followed
// by a "|" for every codec it will accept in that container.
type directPlayProfile struct {
	container string
	codecs    []string
}

func directPlayProfiles(values []string) []directPlayProfile {
	profiles := make([]directPlayProfile, 0, len(values))
	for _, value := range values {
		for _, entry := range strings.Split(value, ",") {
			parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(entry)), func(r rune) bool {
				return r == '|'
			})
			if len(parts) == 0 {
				continue
			}

			profiles = append(profiles, directPlayProfile{container: parts[0], codecs: parts[1:]})
		}
	}

	return profiles
}

// An unprobed source has no codec to compare, which direct plays rather than
// holding back a file the client most likely can play.
func playable(profiles []directPlayProfile, container, codec string) bool {
	for _, profile := range profiles {
		if profile.container != container {
			continue
		}
		if len(profile.codecs) == 0 || codec == "" || slices.Contains(profile.codecs, codec) {
			return true
		}
	}

	return false
}

func describe(container, codec string) string {
	if codec == "" {
		return container
	}

	return container + "|" + codec
}

func sourceContainer(item *items.Item) string {
	if item.Container != "" {
		return strings.ToLower(item.Container)
	}

	return strings.TrimPrefix(strings.ToLower(filepath.Ext(item.Path)), ".")
}

func isStatic(r *http.Request) bool {
	value, err := strconv.ParseBool(r.URL.Query().Get("static"))

	return err == nil && value
}
