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

	query := r.URL.RawQuery
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	fmt.Fprintf(&b, "#EXT-X-VERSION:%d\n", playlistVersion)
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", (step+time.Second-1)/time.Second)
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
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n", piece.Length().Seconds())
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
	segment := transcode.Segment{Path: path, Encode: true}
	var windows []transcode.Window
	if playlist == mainPlaylist {
		segment.Track = transcode.TrackVideo
		segment.Codec = codecOf(items.Picture(source))
		segment.MaxHeight = transcode.MaxHeight
		segment.Transfer = items.HDRTransfer(source)
		windows = transcode.EncodedWindows(runtime)
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
