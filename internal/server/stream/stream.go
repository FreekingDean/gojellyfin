package stream

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
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

type Handler struct {
	sessions *sessions.Service
	items    *items.Service
}

func New(sessions *sessions.Service, items *items.Service) *Handler {
	return &Handler{sessions: sessions, items: items}
}

// Registered ahead of the generated routes because the generated response type
// always writes a complete 200, which would break seeking.
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	item, ok := h.item(w, r)
	if !ok {
		return
	}

	container := r.PathValue("container")
	if container != "" && !isStatic(r) && !strings.EqualFold(container, sourceContainer(item)) {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	h.serveFile(w, r, item)
}

// Upstream chooses between direct play, remux and transcode from the containers
// the client declares it can play. Without an encoder only direct play is on
// offer, so anything else is refused rather than answered with the wrong bytes.
func (h *Handler) ServeUniversal(w http.ResponseWriter, r *http.Request) {
	item, ok := h.item(w, r)
	if !ok {
		return
	}

	profiles := directPlayProfiles(r.URL.Query()["container"])
	if len(profiles) > 0 && !h.directPlays(r.Context(), item, profiles) {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	h.serveFile(w, r, item)
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

// Clients declare each playable format as "container|codec", where the codec
// half is optional.
type directPlayProfile struct {
	container string
	codec     string
}

func directPlayProfiles(values []string) []directPlayProfile {
	profiles := make([]directPlayProfile, 0, len(values))
	for _, value := range values {
		for _, entry := range strings.Split(value, ",") {
			container, codec, _ := strings.Cut(strings.TrimSpace(entry), "|")
			if container == "" {
				continue
			}

			profiles = append(profiles, directPlayProfile{
				container: strings.ToLower(container),
				codec:     strings.ToLower(codec),
			})
		}
	}

	return profiles
}

func (h *Handler) directPlays(ctx context.Context, item *items.Item, profiles []directPlayProfile) bool {
	container := sourceContainer(item)
	codec, err := h.items.AudioCodec(ctx, item.ID)
	if err != nil {
		return false
	}

	for _, profile := range profiles {
		if profile.container != container {
			continue
		}
		if profile.codec == "" || codec == "" || profile.codec == codec {
			return true
		}
	}

	return false
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
