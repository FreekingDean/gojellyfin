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

func (f *fixture) stream(t *testing.T, id uuid.UUID, container, query string) *httptest.ResponseRecorder {
	t.Helper()

	target := "/Videos/" + id.String() + "/stream"
	if container != "" {
		target += "." + container
	}

	request := f.get(t, http.MethodGet, target+"?"+query+"&", id)
	request.SetPathValue("container", container)
	recorder := httptest.NewRecorder()

	f.handler.Serve(recorder, request)

	return recorder
}

func TestHandler_serveRemux(t *testing.T) {
	t.Run("converts the audio the url asks for", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addRip(t)

		recorder := fixture.stream(t, id, "mp4", "container=mp4&audioCodec=aac")

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}
		if got := recorder.Header().Get("Content-Type"); got != "video/mp4" {
			t.Errorf("content type = %q, want video/mp4", got)
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

	t.Run("copies the audio the url names as the source's own", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addPlayableAudioRip(t)

		recorder := fixture.stream(t, id, "mp4", "container=mp4&audioCodec=aac")

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}

		probed := decodable(t, recorder.Body.Bytes(), "out.mp4")
		if !strings.Contains(probed.Format.FormatName, "mp4") {
			t.Errorf("container = %q, want mp4", probed.Format.FormatName)
		}
	})

	t.Run("hands over the file when the url asks for no other container", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addRip(t)

		recorder := fixture.stream(t, id, "", "")

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}

		probed := decodable(t, recorder.Body.Bytes(), "out.mkv")
		for _, stream := range probed.Streams {
			if stream.CodecType == "audio" && stream.CodecName != "ac3" {
				t.Errorf("audio is %q, want the ac3 source passed through", stream.CodecName)
			}
		}
	})

	t.Run("hands over the file when the url names the container it is already in", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addPlayableAudioRip(t)

		recorder := fixture.stream(t, id, "mkv", "container=mkv&audioCodec=aac")

		probed := decodable(t, recorder.Body.Bytes(), "out.mkv")
		if strings.Contains(probed.Format.FormatName, "mp4") {
			t.Error("a file the url asked for as it is was remuxed anyway")
		}
	})

	t.Run("refuses a container it cannot write with no encoder", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t)

		if got := fixture.stream(t, id, "mp4", "container=mp4").Code; got != http.StatusUnsupportedMediaType {
			t.Errorf("status = %d, want %d", got, http.StatusUnsupportedMediaType)
		}
	})
}
