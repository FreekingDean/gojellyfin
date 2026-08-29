package stream

import (
	"context"
	"fmt"
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

func (f *fixture) encode(t *testing.T, name, audio string, height int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=size=%dx%d:rate=15:duration=3", height*4/3, height),
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", audio,
		"-shortest",
		path,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build an h264/%s source: %v: %s", audio, err, output)
	}

	return path
}

func (f *fixture) addRip(t *testing.T, audio string) uuid.UUID {
	t.Helper()

	item, err := f.items.SaveScanned(context.Background(), items.Scanned{
		LibraryID:    f.library,
		Kind:         itemmodal.KindMovie,
		Key:          "movie:" + audio,
		Name:         "rip.mkv",
		SortName:     "rip.mkv",
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	f.addCopy(t, item.ID, "rip.mkv", audio, 240)

	return item.ID
}

func (f *fixture) addCopy(t *testing.T, id uuid.UUID, name, audio string, height int) *items.MediaSource {
	t.Helper()

	ctx := context.Background()
	item, err := f.items.ItemByID(ctx, id)
	if err != nil {
		t.Fatalf("failed to read the item: %v", err)
	}

	source := f.source(t, id, f.encode(t, name, audio, height))
	err = f.items.SaveProbe(ctx, item, source, items.Probe{
		Container: "mkv",
		Streams: []items.Stream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: "h264", Height: int32(height), Width: int32(height * 4 / 3)},
			{Index: 1, Kind: streammodal.KindAudio, Codec: audio},
		},
	})
	if err != nil {
		t.Fatalf("failed to probe %q: %v", name, err)
	}

	return source
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
		id := fixture.addRip(t, "ac3")

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
		id := fixture.addRip(t, "aac")

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
		id := fixture.addRip(t, "ac3")

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
		id := fixture.addRip(t, "aac")

		recorder := fixture.stream(t, id, "mkv", "container=mkv&audioCodec=aac")

		probed := decodable(t, recorder.Body.Bytes(), "out.mkv")
		if strings.Contains(probed.Format.FormatName, "mp4") {
			t.Error("a file the url asked for as it is was remuxed anyway")
		}
	})

	t.Run("serves the version the url names, not the one it would have picked", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addRip(t, "ac3")
		named := fixture.addCopy(t, id, "other.mkv", "aac", 120)

		recorder := fixture.stream(t, id, "mp4", "container=mp4&audioCodec=aac&mediaSourceId="+named.ID.String())

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}

		probed := decodable(t, recorder.Body.Bytes(), "out.mp4")

		var height int
		var audio string
		for _, stream := range probed.Streams {
			switch stream.CodecType {
			case "video":
				height = int(stream.Height)
			case "audio":
				audio = stream.CodecName
			}
		}

		if height != 120 {
			t.Errorf("the response is %d tall, want the 120 the url named rather than the copy beside it", height)
		}
		if audio != "aac" {
			t.Errorf("audio is %q, want the aac the url named", audio)
		}
	})

	t.Run("refuses a container it cannot write with no encoder", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.addRip(t, "ac3")

		if got := fixture.stream(t, id, "mp4", "container=mp4").Code; got != http.StatusUnsupportedMediaType {
			t.Errorf("status = %d, want %d", got, http.StatusUnsupportedMediaType)
		}
	})
}
