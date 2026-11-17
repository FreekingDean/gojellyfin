package stream

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
)

type onAir struct {
	channel  *channels.Channel
	schedule channels.Schedule
	now      time.Time
}

func (o onAir) Channel(_ context.Context, _ items.Viewer, id uuid.UUID) (*channels.Channel, error) {
	if id != o.channel.ID {
		return nil, channels.ErrNotFound
	}

	return o.channel, nil
}

func (o onAir) Schedule(context.Context, *channels.Channel) (channels.Schedule, error) {
	return o.schedule, nil
}

func (o onAir) Now() time.Time {
	return o.now
}

func (f *fixture) tuneTo(t *testing.T, films ...uuid.UUID) (uuid.UUID, time.Time) {
	t.Helper()

	slots := make([]channels.Slot, len(films))
	for i, film := range films {
		sources, err := f.items.MediaSources(context.Background(), film)
		if err != nil || len(sources) != 1 {
			t.Fatalf("film %s has %d files: %v", film, len(sources), err)
		}
		slots[i] = channels.Slot{ItemID: film, SourceID: sources[0].ID, Length: filmSeconds * time.Second}
	}

	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	channel := &channels.Channel{ID: uuid.New(), Number: 7, Name: "Fixture"}
	now := epoch.Add(filmSeconds*time.Second + 8*time.Second)
	f.handler.channels = onAir{
		channel:  channel,
		schedule: channels.NewSchedule(channelmodel.OrderSequential, 1, epoch, slots),
		now:      now,
	}

	return channel.ID, now
}

func TestHandler_Live(t *testing.T) {
	fixture := newFixture(t)
	fixture.withEncoder(t)
	query := "api_key=" + fixture.token
	first := fixture.addFilm(t, "aac", false)
	second := fixture.addFilm(t, "ac3", false)
	id, _ := fixture.tuneTo(t, first, second)
	values := map[string]string{"itemId": id.String()}

	t.Run("the master playlist names an encoded picture and sound", func(t *testing.T) {
		listed := fixture.hls(t, fixture.handler.ServeMasterPlaylist, "/Videos/"+id.String()+"/master.m3u8?"+query, values)
		if listed.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", listed.Code, listed.Body)
		}

		body := listed.Body.String()
		for _, want := range []string{`CODECS="avc1.640029,mp4a.40.2",AUDIO="audio"`, "\nmain.m3u8?" + query + "\n", `URI="audio.m3u8?` + query + `"`} {
			if !strings.Contains(body, want) {
				t.Errorf("master playlist lacks %q:\n%s", want, body)
			}
		}
	})

	for _, playlist := range []string{mainPlaylist, audioPlaylist} {
		t.Run("the "+playlist+" playlist is live and every piece plays", thePlaylistPlaylistIsLiveAndEveryPiecePlays(fixture, playlist, id, query, values))
	}

	t.Run("a piece is found by its number whenever it is asked for", func(t *testing.T) {
		served := fixture.hls(t, fixture.handler.ServeSegment, "/Videos/"+id.String()+"/hls1/main/0.mp4?"+query, map[string]string{
			"itemId": id.String(), "playlistId": "main", "segmentId": "0", "container": "mp4",
		})
		if served.Code != http.StatusOK || served.Body.Len() == 0 {
			t.Errorf("status = %d with %d bytes", served.Code, served.Body.Len())
		}
	})

	t.Run("an unknown playlist is not found", func(t *testing.T) {
		served := fixture.hls(t, fixture.handler.ServeSegment, "/Videos/"+id.String()+"/hls1/subtitles/0.mp4?"+query, map[string]string{
			"itemId": id.String(), "playlistId": "subtitles", "segmentId": "0", "container": "mp4",
		})
		if served.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", served.Code, http.StatusNotFound)
		}
	})

	t.Run("nothing is answered without a token", func(t *testing.T) {
		listed := fixture.hls(t, fixture.handler.ServeMasterPlaylist, "/Videos/"+id.String()+"/master.m3u8", values)
		if listed.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", listed.Code, http.StatusUnauthorized)
		}
	})

	t.Run("a title beside a channel still plays as itself", func(t *testing.T) {
		listed := fixture.hls(t, fixture.handler.ServeMediaPlaylist, "/Videos/"+first.String()+"/main.m3u8?"+query+"&videoCodec=h264", map[string]string{"itemId": first.String()})
		if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), "#EXT-X-ENDLIST") {
			t.Errorf("status = %d:\n%s", listed.Code, listed.Body)
		}
	})
}

func thePlaylistPlaylistIsLiveAndEveryPiecePlays(fixture *fixture, playlist string, id uuid.UUID, query string, values map[string]string) func(*testing.T) {
	return func(t *testing.T) {
		handler := fixture.handler.ServeMediaPlaylist
		if playlist == audioPlaylist {
			handler = fixture.handler.ServeAudioPlaylist
		}
		listed := fixture.hls(t, handler, "/Videos/"+id.String()+"/"+playlist+".m3u8?"+query, values)
		if listed.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", listed.Code, listed.Body)
		}

		body := listed.Body.String()
		if strings.Contains(body, "#EXT-X-ENDLIST") || strings.Contains(body, "#EXT-X-PLAYLIST-TYPE") {
			t.Errorf("a live playlist ends:\n%s", body)
		}
		for _, want := range []string{"#EXT-X-MEDIA-SEQUENCE:", "#EXT-X-DISCONTINUITY-SEQUENCE:", "\n#EXT-X-DISCONTINUITY\n", "#EXT-X-PROGRAM-DATE-TIME:"} {
			if !strings.Contains(body, want) {
				t.Errorf("live playlist lacks %q:\n%s", want, body)
			}
		}

		header := ""
		pieces := 0
		for _, line := range lines(body) {
			if after, found := strings.CutPrefix(line, "#EXT-X-MAP:URI=\""); found {
				header = fixture.fetch(t, id, strings.TrimSuffix(after, "\""))
				continue
			}
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if header == "" {
				t.Fatalf("%s comes before any header:\n%s", line, body)
			}

			path := filepath.Join(t.TempDir(), strconv.Itoa(pieces)+".mp4")
			joined := append(readAll(t, header), readAll(t, fixture.fetch(t, id, line))...)
			if err := os.WriteFile(path, joined, 0o600); err != nil {
				t.Fatal(err)
			}
			kind := "h264"
			if playlist == audioPlaylist {
				kind = "aac"
			}
			if frames := count(t, path, kind); frames == 0 {
				t.Errorf("%s decodes to no frames", line)
			}
			pieces++
		}
		if pieces < 3 {
			t.Errorf("%d pieces listed, want several:\n%s", pieces, body)
		}
	}
}

func (f *fixture) fetch(t *testing.T, id uuid.UUID, uri string) string {
	t.Helper()

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

	path := filepath.Join(t.TempDir(), "piece")
	if err := os.WriteFile(path, served.Body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func readAll(t *testing.T, path string) []byte {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return body
}

func TestHandler_LiveCopiesAPictureItCanCutOnKeyframes(t *testing.T) {
	fixture := newFixture(t)
	fixture.withEncoder(t)
	query := "api_key=" + fixture.token
	id, _ := fixture.tuneTo(t, fixture.addFilm(t, "aac", true), fixture.addFilm(t, "ac3", true))

	listed := fixture.hls(t, fixture.handler.ServeMediaPlaylist, "/Videos/"+id.String()+"/main.m3u8?"+query, map[string]string{"itemId": id.String()})
	if listed.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", listed.Code, listed.Body)
	}
	body := listed.Body.String()
	if !strings.Contains(body, "#EXTINF:8.000000,") || !strings.Contains(body, "#EXT-X-TARGETDURATION:8\n") {
		t.Fatalf("pieces are not cut on the film's keyframes four seconds apart:\n%s", body)
	}

	header := ""
	longest := 0
	for _, line := range lines(body) {
		if after, found := strings.CutPrefix(line, "#EXT-X-MAP:URI=\""); found {
			header = fixture.fetch(t, id, strings.TrimSuffix(after, "\""))
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		path := filepath.Join(t.TempDir(), "piece.mp4")
		if err := os.WriteFile(path, append(readAll(t, header), readAll(t, fixture.fetch(t, id, line))...), 0o600); err != nil {
			t.Fatal(err)
		}
		longest = max(longest, count(t, path, "h264"))
	}
	if longest != 8*filmRate {
		t.Errorf("the longest piece holds %d frames, want the %d of eight copied seconds", longest, 8*filmRate)
	}
}
