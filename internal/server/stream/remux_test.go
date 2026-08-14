package stream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode/worker"
)

// A rip as they actually arrive: h264 a browser plays beside ac3 it cannot
// decode, which is the whole of the "video works, no sound" report.
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
		Name:         "rip.mkv",
		SortName:     "rip.mkv",
		Path:         path,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = f.items.SaveProbe(context.Background(), item, items.Probe{
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

func TestServeRemuxesAudioABrowserCannotDecode(t *testing.T) {
	fixture := newFixture(t)
	fixture.withWorker(t)
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
}

// The client is believed when it says it can decode the source, so a player
// that handles ac3 still gets the file untouched.
func TestServeLeavesAudioTheClientDeclaresAlone(t *testing.T) {
	if !worker.Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	fixture := newFixture(t)
	fixture.withWorker(t)
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
}

func TestServeDirectPlaysWhenTheAudioIsAlreadyPlayable(t *testing.T) {
	fixture := newFixture(t)
	fixture.withWorker(t)
	id := fixture.addTone(t)

	request := fixture.get(t, http.MethodGet, "/Audio/"+id.String()+"/stream?", id)
	if fixture.handler.needsAudioRemux(request, itemOf(t, fixture, id)) {
		t.Error("an audio item was sent for a video remux")
	}
}

func itemOf(t *testing.T, fixture *fixture, id uuid.UUID) *items.Item {
	t.Helper()

	item, err := fixture.items.ItemByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to read the item: %v", err)
	}

	return item
}
