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

func (f *fixture) withEncoder(t *testing.T) {
	t.Helper()

	if !transcode.Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	f.handler.transcoder = transcode.NewEncoder(2, 0)
}

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

func TestHandler_serveTranscode(t *testing.T) {
	t.Run("universal transcodes", func(t *testing.T) {
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
	})

	t.Run("transcodes the requested container", func(t *testing.T) {
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
	})

	t.Run("universal honours startTimeTicks", func(t *testing.T) {
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
	})

	t.Run("universal refuses a container nothing can write", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addTone(t)

		recorder := httptest.NewRecorder()
		target := "/Audio/" + id.String() + "/universal?container=wma&"

		fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, id))

		if recorder.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
		}
	})

	t.Run("universal answers busy when every slot is taken", func(t *testing.T) {
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
	})
}
