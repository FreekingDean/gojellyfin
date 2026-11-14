package stream

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/reports"
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

type Transcoder interface {
	Enabled() bool
	Open(ctx context.Context, spec transcode.Spec) (io.ReadCloser, error)
	Stall() time.Duration
}

type Reports interface {
	File(ctx context.Context, filing reports.Filing) error
}

type Handler struct {
	sessions   *sessions.Service
	items      *items.Service
	files      *filesystem.Service
	transcoder Transcoder
	reports    Reports
}

func New(
	sessions *sessions.Service,
	items *items.Service,
	files *filesystem.Service,
	transcoder Transcoder,
	reports *reports.Service,
) *Handler {
	return &Handler{sessions: sessions, items: items, files: files, transcoder: transcoder, reports: reports}
}

func (h *Handler) broken(r *http.Request, source *items.MediaSource, reason error) {
	err := h.reports.File(context.WithoutCancel(r.Context()), reports.Filing{
		ItemID: source.ItemID,
		FileID: &source.ID,
		Kind:   reports.KindUnplayable,
		Origin: reports.OriginPlayback,
		Note:   reason.Error(),
	})
	if err != nil {
		log.Printf("failed to report %s as broken: %v", source.Path, err)
	}
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	item, source, path, ok := h.item(w, r)
	if !ok {
		return
	}

	container := items.Container(source)
	requested := r.PathValue("container")
	if requested == "" || isStatic(r) || (strings.EqualFold(requested, container) && !converts(r, item, source)) {
		h.serveFile(w, r, source, path)
		return
	}

	if items.IsAudio(item) {
		if h.serveTranscode(w, r, source, path, requested) {
			return
		}
	} else if h.serveRemux(w, r, source, path) {
		return
	}

	unsupported(w, r, container, requested)
}

func (h *Handler) StopEncodings(w http.ResponseWriter, r *http.Request) {
	if !h.authenticated(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ServeUniversal(w http.ResponseWriter, r *http.Request) {
	item, _, _, ok := h.item(w, r)
	if !ok {
		return
	}

	plan, err := h.items.SourceFor(r.Context(), item, universal(r))
	if errors.Is(err, items.ErrNoPlayable) {
		unsupported(w, r, "refused by every file", strings.Join(r.URL.Query()["container"], ","))
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	path, ok := h.resolve(w, r, plan.Source)
	if !ok {
		return
	}

	if plan.Change == items.ChangeNone {
		h.serveFile(w, r, plan.Source, path)
		return
	}
	if h.serveTranscode(w, r, plan.Source, path, plan.Container) {
		return
	}

	unsupported(w, r, describe(items.Container(plan.Source), items.AudioCodec(plan.Source)), plan.Container)
}

func universal(r *http.Request) items.Capabilities {
	query := r.URL.Query()

	var can items.Capabilities
	for _, value := range query["container"] {
		for _, entry := range strings.Split(value, ",") {
			parts := strings.Split(strings.ToLower(strings.TrimSpace(entry)), "|")
			if parts[0] == "" {
				continue
			}

			can.Audio = append(can.Audio, items.Profile{Container: parts[0], AudioCodec: strings.Join(parts[1:], ",")})
		}
	}

	requested := strings.ToLower(strings.TrimSpace(query.Get("transcodingContainer")))
	if requested != "" && !strings.EqualFold(query.Get("transcodingProtocol"), "hls") {
		can.AudioEncodes = []string{requested}
	}

	return can
}

func (h *Handler) serveTranscode(w http.ResponseWriter, r *http.Request, source *items.MediaSource, path, container string) bool {
	if !h.transcoder.Enabled() || !transcode.Supported(container) {
		return false
	}

	return h.relay(w, r, source, transcode.Spec{
		Path:       path,
		Container:  strings.ToLower(container),
		Bitrate:    audioBitrate(r),
		StartTicks: startTicks(r),
	})
}

func (h *Handler) serveRemux(w http.ResponseWriter, r *http.Request, source *items.MediaSource, path string) bool {
	if !h.transcoder.Enabled() {
		return false
	}

	asked := strings.TrimSpace(r.URL.Query().Get("audioCodec"))
	container := r.PathValue("container")
	if !transcode.CarriesVideo(container) {
		container = transcode.VideoContainer
	}

	query := r.URL.Query()
	spec := transcode.Spec{
		Path:       path,
		Container:  strings.ToLower(container),
		Bitrate:    audioBitrate(r),
		StartTicks: startTicks(r),
		Video:      true,
		CopyAudio:  asked != "" && strings.EqualFold(asked, items.AudioCodec(source)),
		VideoCodec: strings.ToLower(strings.TrimSpace(query.Get("videoCodec"))),
	}
	if spec.VideoCodec != "" {
		spec.MaxHeight = maxHeight(r)
		spec.Transfer = items.HDRTransfer(source)
	}

	return h.relay(w, r, source, spec)
}

func converts(r *http.Request, item *items.Item, source *items.MediaSource) bool {
	query := r.URL.Query()
	if query.Get("videoCodec") != "" {
		return true
	}
	asked := strings.TrimSpace(query.Get("audioCodec"))

	return !items.IsAudio(item) && asked != "" && !strings.EqualFold(asked, items.AudioCodec(source))
}

func maxHeight(r *http.Request) int32 {
	height, err := strconv.ParseInt(r.URL.Query().Get("maxHeight"), 10, 32)
	if err != nil || height <= 0 {
		return transcode.MaxHeight
	}

	return int32(min(height, transcode.MaxHeight))
}

func (h *Handler) relay(w http.ResponseWriter, r *http.Request, source *items.MediaSource, spec transcode.Spec) bool {
	container := spec.Container

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	output, err := h.transcoder.Open(ctx, spec)
	if err != nil {
		log.Printf("failed to transcode %s to %s: %v", source.Path, container, err)
		if errors.Is(err, transcode.ErrBusy) {
			busy(w)

			return true
		}
		if !errors.Is(err, transcode.ErrNotAvailable) && r.Context().Err() == nil {
			h.broken(r, source, err)
		}

		return false
	}

	w.Header().Set("Content-Type", transcode.ContentType(container))
	w.Header().Set("Accept-Ranges", "none")
	w.WriteHeader(http.StatusOK)

	kill := func() {
		log.Printf("transcode of %s moved nothing in %s", source.Path, h.transcoder.Stall())
		cancel()
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now())
	}

	stalling := transcode.UntilStalled(ctx, output, h.transcoder.Stall(), kill)
	relayed, err := transcode.Relay(w, stalling)
	if closed := output.Close(); err == nil {
		err = closed
	}
	if err != nil {
		log.Printf("transcode of %s to %s stopped after %d bytes: %v", source.Path, container, relayed, err)
		panic(http.ErrAbortHandler)
	}

	return true
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

func busy(w http.ResponseWriter) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	http.Error(w, "every transcoder is busy", http.StatusServiceUnavailable)
}

func unsupported(w http.ResponseWriter, r *http.Request, source, requested string) {
	log.Printf("no direct play for %s: the source is %s, the client accepts %s", r.URL.Path, source, requested)
	http.Error(w, "no direct play: the source is "+source, http.StatusUnsupportedMediaType)
}

func (h *Handler) item(w http.ResponseWriter, r *http.Request) (*items.Item, *items.MediaSource, string, bool) {
	if !h.authenticated(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, nil, "", false
	}

	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, nil, "", false
	}

	item, err := h.items.ItemByID(r.Context(), items.Everyone, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return nil, nil, "", false
	}

	sources, err := h.items.MediaSources(r.Context(), item.ID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return nil, nil, "", false
	}

	source := items.BestSource(sources)
	if asked, err := uuid.Parse(r.URL.Query().Get("mediaSourceId")); err == nil {
		for _, candidate := range sources {
			if candidate.ID == asked {
				source = candidate
			}
		}
	}
	if source == nil {
		w.WriteHeader(http.StatusNotFound)
		return nil, nil, "", false
	}

	path, ok := h.resolve(w, r, source)
	if !ok {
		return nil, nil, "", false
	}

	return item, source, path, true
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request, source *items.MediaSource) (string, bool) {
	path, err := h.files.Resolve(source.Path)
	if err != nil {
		log.Printf("refusing %s: %q is outside the directories this server serves", r.URL.Path, source.Path)
		w.WriteHeader(http.StatusNotFound)

		return "", false
	}

	return path, true
}

func (h *Handler) authenticated(r *http.Request) bool {
	token := middleware.TokenFrom(r)
	if token == "" {
		return false
	}
	_, err := h.sessions.ByToken(r.Context(), token)

	return err == nil
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, source *items.MediaSource, path string) {
	file, err := os.Open(path)
	if err != nil {
		h.broken(r, source, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if contentType, ok := contentTypes[strings.ToLower(filepath.Ext(path))]; ok {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func describe(container, codec string) string {
	if codec == "" {
		return container
	}

	return container + "|" + codec
}

func isStatic(r *http.Request) bool {
	value, err := strconv.ParseBool(r.URL.Query().Get("static"))

	return err == nil && value
}
