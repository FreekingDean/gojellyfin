package stream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

func (f *fixture) addPlayableAudioRip(t *testing.T) uuid.UUID {
	t.Helper()

	path := filepath.Join(t.TempDir(), "playable.mkv")
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15:duration=3",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		path,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build an h264/aac source: %v: %s", err, output)
	}

	item, err := f.items.SaveScanned(context.Background(), items.Scanned{
		LibraryID:    f.library,
		Kind:         itemmodal.KindMovie,
		Key:          "movie:playable",
		Name:         "playable.mkv",
		SortName:     "playable.mkv",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = f.items.SaveProbe(context.Background(), item, f.source(t, item.ID, path), items.Probe{
		Container: "mkv",
		Streams: []items.Stream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
			{Index: 1, Kind: streammodal.KindAudio, Codec: "aac"},
		},
	})
	if err != nil {
		t.Fatalf("failed to probe the source: %v", err)
	}

	return item.ID
}

func (f *fixture) addRip(t *testing.T) uuid.UUID {
	t.Helper()

	path := filepath.Join(t.TempDir(), "rip.mkv")
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15:duration=3",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "ac3",
		"-shortest",
		path,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build an h264/ac3 source: %v: %s", err, output)
	}

	item, err := f.items.SaveScanned(context.Background(), items.Scanned{
		LibraryID:    f.library,
		Kind:         itemmodal.KindMovie,
		Key:          "movie:rip",
		Name:         "rip.mkv",
		SortName:     "rip.mkv",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = f.items.SaveProbe(context.Background(), item, f.source(t, item.ID, path), items.Probe{
		Container: "mkv",
		Streams: []items.Stream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: "h264"},
			{Index: 1, Kind: streammodal.KindAudio, Codec: "ac3"},
		},
	})
	if err != nil {
		t.Fatalf("failed to probe the source: %v", err)
	}

	return item.ID
}

func TestHandler_serveRemux(t *testing.T) {
	t.Run("remuxes audio a browser cannot decode", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addRip(t)

		recorder := httptest.NewRecorder()
		request := fixture.get(t, http.MethodGet, "/Videos/"+id.String()+"/stream?", id)

		fixture.handler.Serve(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}

		probed := decodable(t, recorder.Body.Bytes(), "out.mp4")

		var video, audio string
		for _, stream := range probed.Streams {
			switch stream.CodecType {
			case "video":
				video = stream.CodecName
			case "audio":
				audio = stream.CodecName
			}
		}

		if video != "h264" {
			t.Errorf("video is %q, want h264 copied through", video)
		}
		if audio != "aac" {
			t.Errorf("audio is %q, want aac", audio)
		}
	})

	t.Run("leaves audio the client declares alone", func(t *testing.T) {
		if !transcode.Available() {
			t.Fatal("ffmpeg is not on PATH")
		}

		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addRip(t)

		recorder := httptest.NewRecorder()
		request := fixture.get(t, http.MethodGet, "/Videos/"+id.String()+"/stream?audioCodec=ac3,aac&", id)

		fixture.handler.Serve(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}

		probed := decodable(t, recorder.Body.Bytes(), "out.mkv")
		for _, stream := range probed.Streams {
			if stream.CodecType == "audio" && stream.CodecName != "ac3" {
				t.Errorf("audio is %q, want the ac3 source passed through", stream.CodecName)
			}
		}
	})

	t.Run("direct plays when the audio is already playable", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addTone(t)

		request := fixture.get(t, http.MethodGet, "/Audio/"+id.String()+"/stream?", id)
		if fixture.handler.needsRemux(request, itemOf(t, fixture, id), sourceOf(t, fixture, id)) {
			t.Error("an audio item was sent for a video remux")
		}
	})

	t.Run("remuxes a container a browser cannot open", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addPlayableAudioRip(t)

		recorder := httptest.NewRecorder()
		request := fixture.get(t, http.MethodGet, "/Videos/"+id.String()+"/stream?", id)

		fixture.handler.Serve(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}

		probed := decodable(t, recorder.Body.Bytes(), "out.mp4")
		if !strings.Contains(probed.Format.FormatName, "mp4") {
			t.Errorf("container = %q, want mp4 — the mkv was served as it is", probed.Format.FormatName)
		}
	})

	t.Run("remuxes the container the transcoding url asks for", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addRip(t)

		recorder := httptest.NewRecorder()
		target := "/Videos/" + id.String() + "/stream.mp4?container=mp4&playSessionId=abc&"
		request := fixture.get(t, http.MethodGet, target, id)
		request.SetPathValue("container", "mp4")

		fixture.handler.Serve(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}
		if got := recorder.Header().Get("Content-Type"); got != "video/mp4" {
			t.Errorf("content type = %q, want video/mp4", got)
		}

		probed := decodable(t, recorder.Body.Bytes(), "out.mp4")
		if !strings.Contains(probed.Format.FormatName, "mp4") {
			t.Errorf("container = %q, want mp4", probed.Format.FormatName)
		}

		var video, audio string
		for _, stream := range probed.Streams {
			switch stream.CodecType {
			case "video":
				video = stream.CodecName
			case "audio":
				audio = stream.CodecName
			}
		}

		if video != "h264" {
			t.Errorf("video is %q, want h264 copied through", video)
		}
		if audio != "aac" {
			t.Errorf("audio is %q, want aac", audio)
		}
	})

	t.Run("refuses a container it cannot remux into", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addRip(t)

		recorder := httptest.NewRecorder()
		request := fixture.get(t, http.MethodGet, "/Videos/"+id.String()+"/stream.webm?", id)
		request.SetPathValue("container", "webm")

		fixture.handler.Serve(recorder, request)

		if recorder.Code != http.StatusUnsupportedMediaType {
			t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
		}
	})

	t.Run("leaves a container the client declares alone", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addPlayableAudioRip(t)

		recorder := httptest.NewRecorder()
		request := fixture.get(t, http.MethodGet, "/Videos/"+id.String()+"/stream?container=mkv&", id)

		fixture.handler.Serve(recorder, request)

		probed := decodable(t, recorder.Body.Bytes(), "out.mkv")
		if strings.Contains(probed.Format.FormatName, "mp4") {
			t.Error("a declared container was remuxed anyway")
		}
	})
}

func sourceOf(t *testing.T, fixture *fixture, id uuid.UUID) *items.MediaSource {
	t.Helper()

	sources, err := fixture.items.MediaSources(context.Background(), id)
	if err != nil || len(sources) == 0 {
		t.Fatalf("failed to read the source: %v", err)
	}

	return sources[0]
}

func itemOf(t *testing.T, fixture *fixture, id uuid.UUID) *items.Item {
	t.Helper()

	item, err := fixture.items.ItemByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to read the item: %v", err)
	}

	return item
}
