package stream

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

const retryAfterSeconds = 10

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
	Stall() time.Duration
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

	if h.needsAudioRemux(r, item) && h.serveRemux(w, r, item) {
		return
	}

	h.serveFile(w, r, item)
}

// Chromium demuxes Matroska but ships no AC-3, E-AC-3, DTS or TrueHD decoder,
// so a rip plays its picture and nothing else. Only the audio is re-encoded;
// the video is copied.
var browserAudio = map[string]bool{
	"aac":       true,
	"mp3":       true,
	"opus":      true,
	"vorbis":    true,
	"flac":      true,
	"pcm_s16le": true,
	"pcm_s24le": true,
}

func (h *Handler) needsAudioRemux(r *http.Request, item *items.Item) bool {
	if items.IsAudio(item) || isStatic(r) || !h.transcoder.Enabled() {
		return false
	}

	codec, err := h.items.AudioCodec(r.Context(), item.ID)
	if err != nil {
		log.Printf("failed to read the audio codec of %s: %v", item.Path, err)

		return false
	}
	if codec == "" {
		return false
	}

	return !acceptedAudio(r)[strings.ToLower(codec)]
}

// A client that says what it can decode is believed; one that says nothing is
// assumed to be a browser, because silence is a worse answer than an encode
// nobody needed.
func acceptedAudio(r *http.Request) map[string]bool {
	raw := r.URL.Query().Get("audioCodec")
	if raw == "" {
		return browserAudio
	}

	accepted := make(map[string]bool)
	for _, codec := range strings.Split(raw, ",") {
		if codec = strings.ToLower(strings.TrimSpace(codec)); codec != "" {
			accepted[codec] = true
		}
	}

	return accepted
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

// Reports whether the response was answered here. Everything that can fail is
// done before the first byte reaches the client, so the caller can still refuse
// with a status when this comes back false.
func (h *Handler) serveTranscode(w http.ResponseWriter, r *http.Request, item *items.Item, accepted []string) bool {
	if !h.transcoder.Enabled() || !items.IsAudio(item) {
		return false
	}

	container := transcode.Choose(candidates(r, accepted))
	if container == "" {
		return false
	}

	return h.relay(w, r, item, transcode.Spec{
		Path:       item.Path,
		Container:  container,
		Bitrate:    audioBitrate(r),
		StartTicks: startTicks(r),
	})
}

// The video is copied rather than encoded, so this costs a mux and an audio
// encode rather than a transcode.
func (h *Handler) serveRemux(w http.ResponseWriter, r *http.Request, item *items.Item) bool {
	return h.relay(w, r, item, transcode.Spec{
		Path:       item.Path,
		Container:  transcode.VideoContainer,
		Bitrate:    audioBitrate(r),
		StartTicks: startTicks(r),
		Video:      true,
	})
}

func (h *Handler) relay(w http.ResponseWriter, r *http.Request, item *items.Item, spec transcode.Spec) bool {
	container := spec.Container

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	output, err := h.transcoder.Open(ctx, spec)
	if err != nil {
		log.Printf("failed to transcode %s to %s: %v", item.Path, container, err)
		if errors.Is(err, transcode.ErrBusy) {
			busy(w)

			return true
		}

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

	// Cancelling reaps ffmpeg, and the deadline ends a write already blocked on
	// a client that stopped acknowledging, which cancelling on its own cannot.
	kill := func() {
		log.Printf("transcode of %s moved nothing in %s", item.Path, h.transcoder.Stall())
		cancel()
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now())
	}

	stalling := transcode.UntilStalled(ctx, output, h.transcoder.Stall(), kill)
	if _, err := transcode.Relay(w, stalling); err != nil {
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

// Every encoder is busy with someone else, which passes: the spec answers these
// operations with a 503 and a Retry-After and has no 415 at all, so the client
// is told to come back rather than that its device cannot play this.
func busy(w http.ResponseWriter) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	http.Error(w, "every transcoder is busy", http.StatusServiceUnavailable)
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
