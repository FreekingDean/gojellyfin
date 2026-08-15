package stream

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

const toneSeconds = 3

// Real ffmpeg in this process, so what comes back is what a client would get
// rather than a stub's bytes.
func (f *fixture) withEncoder(t *testing.T) {
	t.Helper()

	if !transcode.Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	f.handler.transcoder = transcode.NewEncoder(2, 0)
}

// A flac the client in these tests never declares, which is what makes the
// server reach for an encoder instead of the file.
func (f *fixture) tonePath(t *testing.T, id uuid.UUID) string {
	t.Helper()

	sources, err := f.items.MediaSources(context.Background(), id)
	if err != nil || len(sources) == 0 {
		t.Fatalf("failed to load the source: %v", err)
	}

	return sources[0].Path
}

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
		Key:          "audio:tone",
		Name:         "tone.flac",
		SortName:     "tone.flac",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = f.items.SaveProbe(context.Background(), item, f.source(t, item.ID, path), items.Probe{
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
	fixture.withEncoder(t)
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
	fixture.withEncoder(t)
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
	fixture.withEncoder(t)
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
	fixture.withEncoder(t)
	id := fixture.addTone(t)

	recorder := httptest.NewRecorder()
	target := "/Audio/" + id.String() + "/universal?container=wma&"

	fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, id))

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
	}
}

// Every slot on this pod being taken is temporary, so the client is told when
// to come back rather than that its device cannot play this. It is also the
// whole of the load balancing: the gateway reads the refusal and tries another
// pod, with no pool and no shared state.
func TestServeUniversalAnswersBusyWhenEverySlotIsTaken(t *testing.T) {
	fixture := newFixture(t)
	fixture.withEncoder(t)
	id := fixture.addTone(t)

	encoder := transcode.NewEncoder(1, 0)
	fixture.handler.transcoder = encoder

	held, err := encoder.Open(context.Background(), transcode.Spec{
		Path:      fixture.tonePath(t, id),
		Container: "mp3",
	})
	if err != nil {
		t.Fatalf("failed to take the only slot: %v", err)
	}
	t.Cleanup(func() { _ = held.Close() })

	recorder := httptest.NewRecorder()
	target := "/Audio/" + id.String() + "/universal?container=mp3&"

	fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, id))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if got := recorder.Header().Get("Retry-After"); got != "10" {
		t.Errorf("retry after = %q, want 10", got)
	}
}
