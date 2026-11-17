package stream

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

const (
	headerSuffix  = "-header"
	livePieces    = 5
	programLayout = "2006-01-02T15:04:05.000Z07:00"
	tick          = 100 * time.Nanosecond
)

func (h *Handler) tune(w http.ResponseWriter, r *http.Request) (*channels.Schedule, bool) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil || h.channels == nil {
		return nil, false
	}

	channel, err := h.channels.Channel(r.Context(), items.Everyone, id)
	if errors.Is(err, channels.ErrNotFound) {
		return nil, false
	}
	if !h.authenticated(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, true
	}
	if err != nil {
		log.Printf("failed to tune %s: %v", r.URL.Path, err)
		w.WriteHeader(http.StatusInternalServerError)

		return nil, true
	}

	schedule, err := h.channels.Schedule(r.Context(), channel)
	if err != nil {
		log.Printf("failed to schedule %s: %v", r.URL.Path, err)
		w.WriteHeader(http.StatusInternalServerError)

		return nil, true
	}
	if schedule.Empty() {
		log.Printf("nothing airs on %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)

		return nil, true
	}

	return &schedule, true
}

func (h *Handler) serveLiveMaster(w http.ResponseWriter, r *http.Request) {
	query := r.URL.RawQuery

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	fmt.Fprintf(&b, "#EXT-X-VERSION:%d\n", playlistVersion)
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=%q,NAME=\"Audio\",DEFAULT=YES,AUTOSELECT=YES,URI=%q\n", audioGroup, "audio.m3u8?"+query)
	fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,CODECS=\"%s,%s\",AUDIO=%q\n", encodedBandwidth+transcode.AudioBitrate, encodedVideoTag, encodedAudioTag, audioGroup)
	b.WriteString("main.m3u8?" + query + "\n")

	writePlaylist(w, r, b.String())
}

func (h *Handler) serveLivePlaylist(w http.ResponseWriter, r *http.Request, schedule *channels.Schedule, playlist string) {
	step, known := liveStep(playlist)
	if !known {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	now := h.channels.Now()
	pieces := slices.DeleteFunc(schedule.Pieces(step, now.Add(-livePieces*step), now.Add(time.Nanosecond)), func(piece channels.Piece) bool {
		return piece.Sequence < 0
	})
	if len(pieces) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	lengths := h.pieceLengths(r, playlist, pieces)
	longest := step
	for _, length := range lengths {
		longest = max(longest, length)
	}

	query := r.URL.RawQuery
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	fmt.Fprintf(&b, "#EXT-X-VERSION:%d\n", playlistVersion)
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", (longest+time.Second-1)/time.Second)
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", pieces[0].Sequence)
	fmt.Fprintf(&b, "#EXT-X-DISCONTINUITY-SEQUENCE:%d\n", pieces[0].Number)
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	for i, piece := range pieces {
		if i > 0 && piece.Number != pieces[i-1].Number {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		if i == 0 || piece.Number != pieces[i-1].Number {
			fmt.Fprintf(&b, "#EXT-X-MAP:URI=\"hls1/%s%s/%d.%s?%s\"\n", playlist, headerSuffix, piece.Sequence, segmentContainer, query)
		}
		fmt.Fprintf(&b, "#EXT-X-PROGRAM-DATE-TIME:%s\n", piece.Begins.UTC().Format(programLayout))
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n", lengths[i].Seconds())
		fmt.Fprintf(&b, "hls1/%s/%d.%s?%s\n", playlist, piece.Sequence, segmentContainer, query)
	}

	writePlaylist(w, r, b.String())
}

func (h *Handler) serveLiveSegment(w http.ResponseWriter, r *http.Request, schedule *channels.Schedule) {
	playlist, header := strings.CutSuffix(strings.ToLower(r.PathValue("playlistId")), headerSuffix)
	step, known := liveStep(playlist)
	sequence, err := strconv.ParseInt(r.PathValue("segmentId"), 10, 64)
	if !known || err != nil || !strings.EqualFold(r.PathValue("container"), segmentContainer) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	piece, found := schedule.Piece(step, sequence)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	source, path, ok := h.aired(w, r, piece)
	if !ok {
		return
	}

	runtime := int64(piece.Slot.Length / tick)
	segment := transcode.Segment{Path: path, Encode: true, Preset: transcode.LivePreset}
	var windows []transcode.Window
	if playlist == mainPlaylist {
		segment.Track = transcode.TrackVideo
		segment.Codec = codecOf(items.Picture(source))
		var copied bool
		windows, copied = h.pictureWindows(r, source, runtime)
		if copied {
			segment.Encode = false
		} else {
			segment.MaxHeight = transcode.MaxHeight
			segment.Transfer = items.HDRTransfer(source)
		}
	} else {
		segment.Track = transcode.TrackAudio
		segment.Codec = codecOf(items.Sound(source))
		windows = transcode.AudioWindows(runtime)
	}

	index := piece.Index
	if header {
		index = 0
	}
	if segment.Codec == "" || index >= len(windows) {
		log.Printf("no segment %d for %s: the file has no %s to cut there", sequence, r.URL.Path, playlist)
		w.WriteHeader(http.StatusNotFound)

		return
	}
	segment.Window = windows[index]
	segment.Last = index == len(windows)-1

	h.cut(w, r, source, segment, header)
}

func (h *Handler) aired(w http.ResponseWriter, r *http.Request, piece channels.Piece) (*items.MediaSource, string, bool) {
	sources, err := h.items.MediaSources(r.Context(), piece.ItemID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return nil, "", false
	}

	for _, source := range sources {
		if source.ID == piece.SourceID {
			path, ok := h.resolve(w, r, source)

			return source, path, ok
		}
	}

	log.Printf("no file %s for %s", piece.SourceID, r.URL.Path)
	w.WriteHeader(http.StatusNotFound)

	return nil, "", false
}

func (h *Handler) pieceLengths(r *http.Request, playlist string, pieces []channels.Piece) []time.Duration {
	lengths := make([]time.Duration, len(pieces))
	windows := map[int64][]transcode.Window{}
	for i, piece := range pieces {
		lengths[i] = piece.Length()
		if playlist != mainPlaylist {
			continue
		}

		snapped, known := windows[piece.Number]
		if !known {
			snapped = h.snapped(r, piece)
			windows[piece.Number] = snapped
		}
		if piece.Index < len(snapped) {
			lengths[i] = time.Duration(snapped[piece.Index].Length()) * tick
		}
	}

	return lengths
}

func (h *Handler) snapped(r *http.Request, piece channels.Piece) []transcode.Window {
	sources, err := h.items.MediaSources(r.Context(), piece.ItemID)
	if err != nil {
		return nil
	}

	for _, source := range sources {
		if source.ID == piece.SourceID {
			windows, copied := h.pictureWindows(r, source, int64(piece.Slot.Length/tick))
			if copied {
				return windows
			}
		}
	}

	return nil
}

func (h *Handler) pictureWindows(r *http.Request, source *items.MediaSource, runtime int64) ([]transcode.Window, bool) {
	if !copiesLive(source) {
		return transcode.EncodedWindows(runtime), false
	}

	keyframes, err := h.items.Keyframes(r.Context(), source.ID)
	if err != nil {
		log.Printf("failed to read the keyframes of %s: %v", source.ID, err)
		return transcode.EncodedWindows(runtime), false
	}

	windows, ok := transcode.SnappedWindows(keyframes, runtime)
	if !ok {
		return transcode.EncodedWindows(runtime), false
	}

	return windows, true
}

var liveProfiles = []string{"baseline", "constrained baseline", "main", "high"}

func copiesLive(source *items.MediaSource) bool {
	picture := items.Picture(source)
	switch {
	case picture == nil || !strings.EqualFold(picture.Codec, transcode.VideoCodec):
		return false
	case !slices.Contains(liveProfiles, strings.ToLower(picture.Profile)):
		return false
	case picture.PixelFormat != "" && picture.PixelFormat != "yuv420p":
		return false
	case picture.Height == nil || *picture.Height > transcode.MaxHeight:
		return false
	case picture.IsInterlaced || picture.IsAnamorphic:
		return false
	}

	return items.HDRTransfer(source) == ""
}

func liveStep(playlist string) (time.Duration, bool) {
	switch playlist {
	case mainPlaylist:
		return transcode.SegmentTicks * tick, true
	case audioPlaylist:
		return transcode.AudioSegmentTicks * tick, true
	default:
		return 0, false
	}
}
