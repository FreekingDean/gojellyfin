package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

const (
	playlistType      = "application/vnd.apple.mpegurl"
	mainPlaylist      = "main"
	audioPlaylist     = "audio"
	headerSegment     = -1
	encodedBandwidth  = 8_000_000
	fallbackBandwidth = 20_000_000
	segmentContainer  = "mp4"
	playlistVersion   = 7
	audioGroup        = "audio"
	ticksPerSecond    = 10_000_000
	encodedVideoTag   = "avc1.640029"
	encodedAudioTag   = "mp4a.40.2"
)

type rendition struct {
	source  *items.MediaSource
	path    string
	track   transcode.Track
	encode  bool
	codec   string
	windows []transcode.Window
}

func (h *Handler) ServeMasterPlaylist(w http.ResponseWriter, r *http.Request) {
	log.Printf("hls-debug master %s", r.URL.Path)
	_, source, _, ok := h.item(w, r)
	if !ok {
		return
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	fmt.Fprintf(&b, "#EXT-X-VERSION:%d\n", playlistVersion)
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")

	query := r.URL.RawQuery
	sound := items.Sound(source)
	if sound != nil {
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=%q,NAME=\"Audio\",DEFAULT=YES,AUTOSELECT=YES,URI=%q\n", audioGroup, "audio.m3u8?"+query)
	}

	attributes := []string{"BANDWIDTH=" + strconv.FormatInt(bandwidth(r, source), 10)}
	if resolution := resolution(r, source); resolution != "" {
		attributes = append(attributes, "RESOLUTION="+resolution)
	}
	if codecs := codecs(r, source); codecs != "" {
		attributes = append(attributes, "CODECS=\""+codecs+"\"")
	}
	if sound != nil {
		attributes = append(attributes, "AUDIO=\""+audioGroup+"\"")
	}
	fmt.Fprintf(&b, "#EXT-X-STREAM-INF:%s\n", strings.Join(attributes, ","))
	b.WriteString("main.m3u8?" + query + "\n")

	writePlaylist(w, r, b.String())
}

func (h *Handler) ServeMediaPlaylist(w http.ResponseWriter, r *http.Request) {
	h.servePlaylist(w, r, mainPlaylist)
}

func (h *Handler) ServeAudioPlaylist(w http.ResponseWriter, r *http.Request) {
	h.servePlaylist(w, r, audioPlaylist)
}

func (h *Handler) servePlaylist(w http.ResponseWriter, r *http.Request, playlist string) {
	chosen, ok := h.rendition(w, r, playlist)
	if !ok {
		return
	}

	longest := int64(0)
	for _, window := range chosen.windows {
		longest = max(longest, window.Length())
	}

	query := r.URL.RawQuery
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	fmt.Fprintf(&b, "#EXT-X-VERSION:%d\n", playlistVersion)
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", (longest+ticksPerSecond-1)/ticksPerSecond)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=\"hls1/%s/%d.%s?%s\"\n", playlist, headerSegment, segmentContainer, query)
	for i, window := range chosen.windows {
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n", float64(window.Length())/ticksPerSecond)
		fmt.Fprintf(&b, "hls1/%s/%d.%s?%s\n", playlist, i, segmentContainer, query)
	}
	b.WriteString("#EXT-X-ENDLIST\n")

	writePlaylist(w, r, b.String())
}

func (h *Handler) ServeSegment(w http.ResponseWriter, r *http.Request) {
	playlist := strings.ToLower(r.PathValue("playlistId"))
	index, err := strconv.Atoi(r.PathValue("segmentId"))
	if err != nil || index < headerSegment || !strings.EqualFold(r.PathValue("container"), segmentContainer) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	chosen, ok := h.rendition(w, r, playlist)
	if !ok {
		return
	}

	header := index == headerSegment
	if header {
		index = 0
	}
	if index >= len(chosen.windows) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	segment := transcode.Segment{
		Path:   chosen.path,
		Track:  chosen.track,
		Window: chosen.windows[index],
		Last:   index == len(chosen.windows)-1,
		Encode: chosen.encode,
		Codec:  chosen.codec,
	}
	if segment.EncodesVideo() {
		segment.MaxHeight = maxHeight(r)
		segment.Transfer = items.HDRTransfer(chosen.source)
	}

	h.cut(w, r, chosen.source, segment, header)
}

func (h *Handler) rendition(w http.ResponseWriter, r *http.Request, playlist string) (rendition, bool) {
	_, source, path, ok := h.item(w, r)
	if !ok {
		return rendition{}, false
	}

	runtime := int64(0)
	if source.RunTimeTicks != nil {
		runtime = *source.RunTimeTicks
	}
	if runtime <= 0 {
		log.Printf("no segments for %s: the file has no runtime", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)

		return rendition{}, false
	}

	chosen := rendition{source: source, path: path}
	switch playlist {
	case mainPlaylist:
		chosen.track = transcode.TrackVideo
		chosen.codec = codecOf(items.Picture(source))
		chosen.encode = encodesVideo(r)
		if chosen.encode {
			chosen.windows = transcode.EncodedWindows(runtime)
			break
		}

		keyframes, err := h.items.Keyframes(r.Context(), source.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return rendition{}, false
		}
		chosen.windows = transcode.CopiedWindows(keyframes, runtime)
	case audioPlaylist:
		chosen.track = transcode.TrackAudio
		chosen.codec = codecOf(items.Sound(source))
		chosen.encode = encodesAudio(r, chosen.codec)
		chosen.windows = transcode.AudioWindows(runtime)
	default:
		w.WriteHeader(http.StatusNotFound)
		return rendition{}, false
	}

	if len(chosen.windows) == 0 || chosen.codec == "" {
		log.Printf("no segments for %s: the file has no %s to cut", r.URL.Path, playlist)
		w.WriteHeader(http.StatusNotFound)

		return rendition{}, false
	}

	return chosen, true
}

func (h *Handler) cut(w http.ResponseWriter, r *http.Request, source *items.MediaSource, segment transcode.Segment, header bool) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	began := time.Now()
	log.Printf("hls-debug begin %s header=%v %+v", r.URL.Path, header, segment.Window)
	defer func() { log.Printf("hls-debug end %s after %s ctx=%v", r.URL.Path, time.Since(began), r.Context().Err()) }()
	output, err := h.transcoder.Segment(ctx, segment)
	log.Printf("hls-debug opened %s after %s err=%v", r.URL.Path, time.Since(began), err)
	if err != nil {
		log.Printf("failed to cut segment %+v of %s: %v", segment.Window, source.Path, err)
		switch {
		case errors.Is(err, transcode.ErrBusy):
			busy(w)
		case errors.Is(err, transcode.ErrNotAvailable):
			http.Error(w, "segments cannot be cut on this server", http.StatusUnsupportedMediaType)
		default:
			if r.Context().Err() == nil {
				h.broken(r, source, segmented(source, segment), err)
			}
			http.Error(w, "the segment could not be cut", http.StatusInternalServerError)
		}

		return
	}
	defer func() {
		cancel()
		_ = output.Close()
	}()

	stop := func() {
		cancel()
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now())
	}
	defer context.AfterFunc(r.Context(), stop)()
	kill := func() {
		log.Printf("segment %+v of %s moved nothing in %s", segment.Window, source.Path, h.transcoder.Stall())
		stop()
	}
	stalling := transcode.UntilStalled(ctx, output, h.transcoder.Stall(), kill)

	w.Header().Set("Content-Type", segment.ContentType())
	w.WriteHeader(http.StatusOK)

	body := transcode.Flushed(w)
	if header {
		err = transcode.Header(body, stalling)
	} else {
		err = transcode.Fragments(body, stalling, segment.Bounds())
	}
	log.Printf("hls-debug written %s after %s err=%v", r.URL.Path, time.Since(began), err)
	if err != nil {
		log.Printf("segment %+v of %s stopped: %v", segment.Window, source.Path, err)
		panic(http.ErrAbortHandler)
	}
}

func writePlaylist(w http.ResponseWriter, r *http.Request, body string) {
	w.Header().Set("Content-Type", playlistType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, body)
	}
}

func codecOf(stream *items.MediaStream) string {
	if stream == nil {
		return ""
	}

	return strings.ToLower(stream.Codec)
}

func bandwidth(r *http.Request, source *items.MediaSource) int64 {
	if encodesVideo(r) {
		return encodedBandwidth + transcode.AudioBitrate
	}
	if source.Bitrate != nil && *source.Bitrate > 0 {
		return int64(*source.Bitrate)
	}

	return fallbackBandwidth
}

func resolution(r *http.Request, source *items.MediaSource) string {
	picture := items.Picture(source)
	if picture == nil || picture.Width == nil || picture.Height == nil || encodesVideo(r) {
		return ""
	}

	return fmt.Sprintf("%dx%d", *picture.Width, *picture.Height)
}

func codecs(r *http.Request, source *items.MediaSource) string {
	video := encodedVideoTag
	if !encodesVideo(r) {
		video = videoCodecTag(items.Picture(source))
	}
	if video == "" {
		return ""
	}

	sound := items.Sound(source)
	switch {
	case sound == nil:
		return video
	case encodesAudio(r, codecOf(sound)):
		return video + "," + encodedAudioTag
	default:
		return video + "," + audioCodecTag(sound.Codec)
	}
}

func encodesVideo(r *http.Request) bool {
	return strings.TrimSpace(r.URL.Query().Get("videoCodec")) != ""
}

func encodesAudio(r *http.Request, codec string) bool {
	asked := strings.TrimSpace(r.URL.Query().Get("audioCodec"))

	return !transcode.CopiesAudio(codec) || (asked != "" && !strings.EqualFold(asked, codec))
}

var h264Profiles = map[string]int{
	"baseline":              0x42,
	"constrained baseline":  0x42,
	"main":                  0x4d,
	"high":                  0x64,
	"high 10":               0x6e,
	"high 4:2:2":            0x7a,
	"high 4:4:4 predictive": 0xf4,
}

func videoCodecTag(picture *items.MediaStream) string {
	if picture == nil || picture.Level == nil {
		return ""
	}

	switch strings.ToLower(picture.Codec) {
	case "h264":
		profile, known := h264Profiles[strings.ToLower(picture.Profile)]
		if !known {
			return ""
		}
		constraints := 0
		if strings.EqualFold(picture.Profile, "constrained baseline") {
			constraints = 0x40
		}

		return fmt.Sprintf("avc1.%02x%02x%02x", profile, constraints, int(*picture.Level))
	case "hevc":
		profile := 1
		if strings.EqualFold(picture.Profile, "main 10") {
			profile = 2
		}

		return fmt.Sprintf("hvc1.%d.4.L%d.B0", profile, int(*picture.Level))
	default:
		return ""
	}
}

func audioCodecTag(codec string) string {
	switch strings.ToLower(codec) {
	case "aac":
		return "mp4a.40.2"
	case "mp3":
		return "mp4a.40.34"
	case "ac3":
		return "ac-3"
	case "eac3":
		return "ec-3"
	case "opus":
		return "Opus"
	case "flac":
		return "fLaC"
	case "alac":
		return "alac"
	default:
		return ""
	}
}

func segmented(source *items.MediaSource, segment transcode.Segment) items.Playback {
	served := whole(source)
	served.Direct = false
	served.Container = "hls"

	switch {
	case segment.Track == transcode.TrackVideo && segment.Encode:
		served.VideoDirect = false
		served.VideoCodec = transcode.VideoCodec
	case segment.Track == transcode.TrackAudio && segment.Encode:
		served.AudioDirect = false
		served.AudioCodec = transcode.SegmentAudioCodec
	}

	return served
}
