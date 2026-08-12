package stream

import (
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

	container := sourceContainer(item)
	if requested := r.PathValue("container"); requested != "" && !isStatic(r) && !strings.EqualFold(requested, container) {
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
		unsupported(w, r, describe(container, codec), strings.Join(requested, ","))
		return
	}

	h.serveFile(w, r, item)
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
