package stream

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

const (
	filmSeconds = 14
	filmRate    = 15
)

func (f *fixture) addFilm(t *testing.T, audio string, indexed bool) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "film.mkv")
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate="+strconv.Itoa(filmRate)+":duration="+strconv.Itoa(filmSeconds),
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration="+strconv.Itoa(filmSeconds),
		"-c:v", "libx264", "-bf", "2", "-g", strconv.Itoa(4*filmRate), "-keyint_min", strconv.Itoa(4*filmRate), "-sc_threshold", "0",
		"-c:a", audio, path,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build the film: %v: %s", err, output)
	}

	item, err := f.items.SaveScanned(ctx, items.Item{
		Kind:         itemmodel.KindMovie,
		Key:          "movie:hls:" + uuid.NewString(),
		Name:         "film",
		SortName:     "film",
		DateModified: apiutil.Ptr(time.Now()),
	})
	if err != nil {
		t.Fatalf("failed to save the film: %v", err)
	}

	probe := items.MediaSource{
		Container:    "mkv",
		RunTimeTicks: apiutil.Ptr(int64(filmSeconds * 10_000_000)),
		Streams: []*items.MediaStream{
			{Index: 0, Kind: streammodel.KindVideo, Codec: "h264", Profile: "High", Level: apiutil.Ptr(13.0), Width: apiutil.Ptr(int32(320)), Height: apiutil.Ptr(int32(240))},
			{Index: 1, Kind: streammodel.KindAudio, Codec: audio},
		},
	}
	if indexed {
		keyframes, err := ffmpeg.New().Keyframes(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		probe.Keyframes = &items.KeyframeIndex{Ticks: keyframes}
	}

	source := f.source(t, item.ID, path)
	if err := f.items.SaveProbe(ctx, item, source, probe); err != nil {
		t.Fatalf("failed to probe the film: %v", err)
	}

	return item.ID
}

func (f *fixture) hls(t *testing.T, handler http.HandlerFunc, target string, values map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, target, nil)
	for name, value := range values {
		request.SetPathValue(name, value)
	}
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	return recorder
}

func lines(body string) []string {
	var found []string
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		found = append(found, scanner.Text())
	}

	return found
}

func (f *fixture) play(t *testing.T, id uuid.UUID, playlist, query string) string {
	t.Helper()

	handler := f.handler.ServeMediaPlaylist
	if playlist == audioPlaylist {
		handler = f.handler.ServeAudioPlaylist
	}
	listed := f.hls(t, handler, "/Videos/"+id.String()+"/"+playlist+".m3u8?"+query, map[string]string{"itemId": id.String()})
	if listed.Code != http.StatusOK {
		t.Fatalf("%s playlist: status = %d: %s", playlist, listed.Code, listed.Body)
	}

	var uris []string
	for _, line := range lines(listed.Body.String()) {
		if after, found := strings.CutPrefix(line, "#EXT-X-MAP:URI=\""); found {
			uris = append(uris, strings.TrimSuffix(after, "\""))
		} else if line != "" && !strings.HasPrefix(line, "#") {
			uris = append(uris, line)
		}
	}
	if len(uris) < 3 {
		t.Fatalf("%s playlist names %d files, want a header and several segments:\n%s", playlist, len(uris), listed.Body)
	}

	var joined []byte
	for _, uri := range uris {
		parsed, err := url.Parse(uri)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(parsed.Path, "/")
		segment, container, _ := strings.Cut(parts[2], ".")
		served := f.hls(t, f.handler.ServeSegment, "/Videos/"+id.String()+"/"+uri, map[string]string{
			"itemId": id.String(), "playlistId": parts[1], "segmentId": segment, "container": container,
		})
		if served.Code != http.StatusOK {
			t.Fatalf("%s: status = %d: %s", uri, served.Code, served.Body)
		}
		joined = append(joined, served.Body.Bytes()...)
	}

	path := filepath.Join(t.TempDir(), playlist+".mp4")
	if err := os.WriteFile(path, joined, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func count(t *testing.T, path, kind string) int {
	t.Helper()

	output, err := exec.Command("ffprobe", "-v", "error", "-count_frames", "-show_entries", "stream=codec_type,codec_name,nb_read_frames", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("the joined %s does not decode: %v", kind, err)
	}

	fields := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(fields) != 3 || fields[0] != kind {
		t.Fatalf("joined stream = %q, want one %s stream", output, kind)
	}
	frames, err := strconv.Atoi(fields[2])
	if err != nil {
		t.Fatal(err)
	}

	return frames
}

func TestHandler_HLS(t *testing.T) {
	fixture := newFixture(t)
	fixture.withEncoder(t)
	query := "api_key=" + fixture.token

	t.Run("the master playlist names the picture and the sound", theMasterPlaylistNamesThePictureAndTheSound(fixture, query))

	t.Run("a copied picture and sound play end to end", func(t *testing.T) {
		id := fixture.addFilm(t, "aac", true)

		if frames := count(t, fixture.play(t, id, mainPlaylist, query), "h264"); frames != filmSeconds*filmRate {
			t.Errorf("%d frames, want %d", frames, filmSeconds*filmRate)
		}
		if frames := count(t, fixture.play(t, id, audioPlaylist, query), "aac"); frames < filmSeconds*48000/1024-2 {
			t.Errorf("%d audio frames, want about %d", frames, filmSeconds*48000/1024)
		}
	})

	t.Run("an encoded picture and sound play end to end", func(t *testing.T) {
		id := fixture.addFilm(t, "ac3", false)
		encoded := query + "&videoCodec=h264&audioCodec=aac"

		if frames := count(t, fixture.play(t, id, mainPlaylist, encoded), "h264"); frames != filmSeconds*filmRate {
			t.Errorf("%d frames, want %d", frames, filmSeconds*filmRate)
		}
		if frames := count(t, fixture.play(t, id, audioPlaylist, encoded), "aac"); frames < filmSeconds*48000/1024-2 {
			t.Errorf("%d audio frames, want about %d", frames, filmSeconds*48000/1024)
		}
	})

	t.Run("a start cuts the playlists from the window holding it", aStartCutsThePlaylistsFromTheWindowHoldingIt(fixture, query))

	t.Run("a copied picture without a keyframe index has no playlist", func(t *testing.T) {
		id := fixture.addFilm(t, "aac", false)
		listed := fixture.hls(t, fixture.handler.ServeMediaPlaylist, "/Videos/"+id.String()+"/main.m3u8?"+query, map[string]string{"itemId": id.String()})
		if listed.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", listed.Code, http.StatusNotFound)
		}
	})

	t.Run("a segment past the end is not found", func(t *testing.T) {
		id := fixture.addFilm(t, "aac", true)
		served := fixture.hls(t, fixture.handler.ServeSegment, "/Videos/"+id.String()+"/hls1/main/99.mp4?"+query, map[string]string{
			"itemId": id.String(), "playlistId": "main", "segmentId": "99", "container": "mp4",
		})
		if served.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", served.Code, http.StatusNotFound)
		}
	})

	t.Run("nothing is answered without a token", func(t *testing.T) {
		id := fixture.addFilm(t, "aac", true)
		listed := fixture.hls(t, fixture.handler.ServeMasterPlaylist, "/Videos/"+id.String()+"/master.m3u8", map[string]string{"itemId": id.String()})
		if listed.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", listed.Code, http.StatusUnauthorized)
		}
	})
}

func aStartCutsThePlaylistsFromTheWindowHoldingIt(fixture *fixture, query string) func(*testing.T) {
	return func(t *testing.T) {
		id := fixture.addFilm(t, "ac3", false)
		from := query + "&videoCodec=h264&audioCodec=aac&fromTicks=" + strconv.Itoa(7*10_000_000)

		if frames := count(t, fixture.play(t, id, mainPlaylist, from), "h264"); frames != (filmSeconds-6)*filmRate {
			t.Errorf("%d frames, want the %d from the window at six seconds", frames, (filmSeconds-6)*filmRate)
		}
		if frames := count(t, fixture.play(t, id, audioPlaylist, from), "aac"); frames >= filmSeconds*48000/1024-2 {
			t.Errorf("%d audio frames, want fewer than the whole film's", frames)
		}
	}
}

func theMasterPlaylistNamesThePictureAndTheSound(fixture *fixture, query string) func(*testing.T) {
	return func(t *testing.T) {
		id := fixture.addFilm(t, "aac", true)
		listed := fixture.hls(t, fixture.handler.ServeMasterPlaylist, "/Videos/"+id.String()+"/master.m3u8?"+query, map[string]string{"itemId": id.String()})
		if listed.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", listed.Code, listed.Body)
		}
		if got := listed.Header().Get("Content-Type"); got != playlistType {
			t.Errorf("content type = %q", got)
		}

		body := listed.Body.String()
		for _, want := range []string{
			`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="Audio",DEFAULT=YES,AUTOSELECT=YES,URI="audio.m3u8?` + query + `"`,
			`RESOLUTION=320x240,CODECS="avc1.64000d,mp4a.40.2",AUDIO="audio"`,
			"\nmain.m3u8?" + query + "\n",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("master playlist lacks %q:\n%s", want, body)
			}
		}
	}
}

func TestSegmented(t *testing.T) {
	source := &items.MediaSource{
		Container: "mkv",
		Streams: []*items.MediaStream{
			{Kind: streammodel.KindVideo, Codec: "hevc"},
			{Kind: streammodel.KindAudio, Codec: "eac3"},
		},
	}

	for _, tc := range []struct {
		name    string
		segment transcode.Segment
		want    items.Playback
	}{
		{
			name:    "a copied picture keeps the source's codec",
			segment: transcode.Segment{Track: transcode.TrackVideo},
			want:    items.Playback{VideoDirect: true, AudioDirect: true, Container: "hls", VideoCodec: "hevc", AudioCodec: "eac3"},
		},
		{
			name:    "an encoded picture names what it becomes",
			segment: transcode.Segment{Track: transcode.TrackVideo, Encode: true},
			want:    items.Playback{AudioDirect: true, Container: "hls", VideoCodec: transcode.VideoCodec, AudioCodec: "eac3"},
		},
		{
			name:    "encoded sound names what it becomes",
			segment: transcode.Segment{Track: transcode.TrackAudio, Encode: true},
			want:    items.Playback{VideoDirect: true, Container: "hls", VideoCodec: "hevc", AudioCodec: transcode.SegmentAudioCodec},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := segmented(source, tc.segment)
			if got.Direct != tc.want.Direct || got.VideoDirect != tc.want.VideoDirect || got.AudioDirect != tc.want.AudioDirect ||
				got.Container != tc.want.Container || got.VideoCodec != tc.want.VideoCodec || got.AudioCodec != tc.want.AudioCodec {
				t.Errorf("segmented = %+v, want %+v", got, tc.want)
			}
		})
	}
}
