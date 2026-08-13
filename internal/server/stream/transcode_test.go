package stream

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
	"github.com/FreekingDean/gojellyfin/internal/transcode/worker"
)

const toneSeconds = 3

// The whole path, with the pool talking HTTP to a worker that runs ffmpeg, so
// what comes back is what a client would get rather than a stub's bytes.
func (f *fixture) withWorker(t *testing.T) {
	t.Helper()

	if !worker.Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	server := httptest.NewServer(worker.New().Handler())
	t.Cleanup(server.Close)

	t.Setenv("TRANSCODER_WORKERS", server.URL)
	f.handler.transcoder = transcode.NewPool()
}

// A flac the client in these tests never declares, which is what makes the
// server reach for an encoder instead of the file.
func (f *fixture) addTone(t *testing.T) uuid.UUID {
	t.Helper()

	path := filepath.Join(t.TempDir(), "tone.flac")
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=3",
		path,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate the source: %v: %s", err, output)
	}

	item, err := f.items.SaveScanned(context.Background(), items.Scanned{
		LibraryID:    f.library,
		Kind:         itemmodal.KindAudio,
		Name:         "tone.flac",
		SortName:     "tone.flac",
		Path:         path,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = f.items.SaveProbe(context.Background(), item, items.Probe{
		Container: "flac",
		Streams:   []items.Stream{{Kind: streammodal.KindAudio, Codec: "flac"}},
	})
	if err != nil {
		t.Fatalf("failed to probe the source: %v", err)
	}

	return item.ID
}

func decodable(t *testing.T, body []byte, name string) *ffmpeg.Probe {
	t.Helper()

	if len(body) == 0 {
		t.Fatal("the response has no body")
	}

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("failed to write the response: %v", err)
	}

	probed, err := ffmpeg.ProbeFile(context.Background(), path)
	if err != nil {
		t.Fatalf("the response is not decodable: %v", err)
	}

	return probed
}

func TestServeUniversalTranscodes(t *testing.T) {
	fixture := newFixture(t)
	fixture.withWorker(t)
	id := fixture.addTone(t)

	recorder := httptest.NewRecorder()
	target := "/Audio/" + id.String() + "/universal?container=mp3%7Cmp3&"

	fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, id))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	if got := recorder.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Errorf("content type = %q, want audio/mpeg", got)
	}
	if got := recorder.Header().Get("Accept-Ranges"); got != "none" {
		t.Errorf("accept ranges = %q, want none", got)
	}

	probed := decodable(t, recorder.Body.Bytes(), "out.mp3")
	if len(probed.Streams) == 0 || probed.Streams[0].CodecName != "mp3" {
		t.Fatalf("the response is not mp3: %+v", probed.Streams)
	}
	if seconds := probed.Format.Seconds(); math.Abs(seconds-toneSeconds) > 1 {
		t.Errorf("the response is %.2fs long, want about %ds", seconds, toneSeconds)
	}
}

func TestServeTranscodesTheRequestedContainer(t *testing.T) {
	fixture := newFixture(t)
	fixture.withWorker(t)
	id := fixture.addTone(t)

	recorder := httptest.NewRecorder()
	request := fixture.get(t, http.MethodGet, "/Audio/"+id.String()+"/stream.aac?", id)
	request.SetPathValue("container", "aac")

	fixture.handler.Serve(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
	}

	probed := decodable(t, recorder.Body.Bytes(), "out.aac")
	if len(probed.Streams) == 0 || probed.Streams[0].CodecName != "aac" {
		t.Fatalf("the response is not aac: %+v", probed.Streams)
	}
}

func TestServeUniversalHonoursStartTimeTicks(t *testing.T) {
	fixture := newFixture(t)
	fixture.withWorker(t)
	id := fixture.addTone(t)

	recorder := httptest.NewRecorder()
	target := "/Audio/" + id.String() + "/universal?container=mp3&startTimeTicks=20000000&"

	fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, id))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	if seconds := decodable(t, recorder.Body.Bytes(), "out.mp3").Format.Seconds(); seconds > toneSeconds-1 {
		t.Errorf("seeking 2s into a %ds source produced %.2fs", toneSeconds, seconds)
	}
}

// The client asked for a container no encoder here writes, so the refusal has
// to stand rather than become a stream of something else.
func TestServeUniversalRefusesAContainerNothingCanWrite(t *testing.T) {
	fixture := newFixture(t)
	fixture.withWorker(t)
	id := fixture.addTone(t)

	recorder := httptest.NewRecorder()
	target := "/Audio/" + id.String() + "/universal?container=wma&"

	fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, id))

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
	}
}

// A worker that is down has to surface as the refusal the client already knows
// how to read, not as a truncated stream.
func TestServeUniversalRefusesWhenNoWorkerAnswers(t *testing.T) {
	fixture := newFixture(t)
	id := fixture.addTone(t)

	unreachable := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unreachable.Close()

	t.Setenv("TRANSCODER_WORKERS", unreachable.URL)
	fixture.handler.transcoder = transcode.NewPool()

	recorder := httptest.NewRecorder()
	target := "/Audio/" + id.String() + "/universal?container=mp3&"

	fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, id))

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
	}
}

func TestPoolIsDisabledWithoutWorkers(t *testing.T) {
	t.Setenv("TRANSCODER_WORKERS", "")

	pool := transcode.NewPool()
	if pool.Enabled() {
		t.Fatal("the pool is enabled with no workers configured")
	}
	if _, err := pool.Open(context.Background(), transcode.Spec{Path: "/tmp/x.flac", Container: "mp3"}); !errors.Is(err, transcode.ErrNoWorker) {
		t.Fatalf("err = %v, want %v", err, transcode.ErrNoWorker)
	}
}

func TestPoolSpreadsAcrossWorkers(t *testing.T) {
	seen := make(chan string, 4)
	servers := make([]string, 0, 2)
	for range 2 {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen <- r.Host
			http.Error(w, "busy", http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		servers = append(servers, server.URL)
	}

	t.Setenv("TRANSCODER_WORKERS", strings.Join(servers, ","))
	pool := transcode.NewPool()

	if _, err := pool.Open(context.Background(), transcode.Spec{Path: "/tmp/x.flac", Container: "mp3"}); !errors.Is(err, transcode.ErrNoWorker) {
		t.Fatalf("err = %v, want %v", err, transcode.ErrNoWorker)
	}
	close(seen)

	hosts := make(map[string]bool)
	for host := range seen {
		hosts[host] = true
	}
	if len(hosts) != 2 {
		t.Fatalf("tried %d workers, want both", len(hosts))
	}
}
