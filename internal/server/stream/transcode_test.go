package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	reportmodel "github.com/FreekingDean/gojellyfin/internal/store/report"
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

	item, err := f.items.SaveScanned(context.Background(), items.Item{
		Kind:         itemmodel.KindAudio,
		Key:          "audio:tone",
		Name:         "tone.flac",
		SortName:     "tone.flac",
		DateModified: apiutil.Ptr(time.Now()),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = f.items.SaveProbe(context.Background(), item, f.source(t, item.ID, path), items.MediaSource{
		Container: "flac",
		Streams:   []*items.MediaStream{{Kind: streammodel.KindAudio, Codec: "flac"}},
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

	probed, err := ffmpeg.New().ProbeFile(context.Background(), path)
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
		if seconds := probed.Format.Duration; math.Abs(seconds-toneSeconds) > 1 {
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

	t.Run("universal encodes into the transcoding container the client named", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.withEncoder(t)
		id := fixture.addTone(t)

		recorder := httptest.NewRecorder()
		target := "/Audio/" + id.String() + "/universal?container=mp3&transcodingContainer=opus&transcodingProtocol=http&"

		fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, id))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}
		if got := recorder.Header().Get("Content-Type"); got != "audio/opus" {
			t.Errorf("content type = %q, want audio/opus", got)
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
		if seconds := decodable(t, recorder.Body.Bytes(), "out.mp3").Format.Duration; seconds > toneSeconds-1 {
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

		for range transcode.SlotShares {
			held, err := encoder.Open(context.Background(), transcode.Spec{
				Path:      fixture.tonePath(t, id),
				Container: "mp3",
			})
			if err != nil {
				t.Fatalf("failed to take a share of the only slot: %v", err)
			}
			t.Cleanup(func() { _ = held.Close() })
		}

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

type dying struct {
	body []byte
	err  error
}

func (d *dying) Read(b []byte) (int, error) {
	if len(d.body) == 0 {
		return 0, d.err
	}
	n := copy(b, d.body)
	d.body = d.body[n:]

	return n, nil
}

func (d *dying) Close() error {
	return d.err
}

func (f *fixture) streamed(t *testing.T, id uuid.UUID) ([]byte, error) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("itemId", id.String())
		r.SetPathValue("container", "mp3")
		f.handler.Serve(w, r)
	}))
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/Audio/" + id.String() + "/stream.mp3?api_key=" + f.token)
	if err != nil {
		t.Fatalf("failed to request the stream: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	return io.ReadAll(response.Body)
}

func (f *fixture) filed(t *testing.T, id uuid.UUID) []*reports.Report {
	t.Helper()

	found, err := f.client.Report.Query().Where(reportmodel.ItemID(id)).AllModels(context.Background())
	if err != nil {
		t.Fatalf("failed to query the reports: %v", err)
	}

	return found
}

type killed struct {
	ctx  context.Context
	sent bool
}

func (k *killed) Read(b []byte) (int, error) {
	if !k.sent {
		k.sent = true

		return copy(b, "partial"), nil
	}
	<-k.ctx.Done()

	return 0, fmt.Errorf("%w: signal: killed", transcode.ErrExited)
}

func (k *killed) Close() error {
	return nil
}

func TestHandler_relay(t *testing.T) {
	exited := fmt.Errorf("%w: exit status 1: Error while decoding", transcode.ErrExited)

	t.Run("aborts the response and reports the file when ffmpeg dies partway", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.add(t, uuid.NewString()+".flac", "flac")

		fixture.transcoder.enabled = true
		fixture.transcoder.open = func(context.Context, transcode.Spec) (io.ReadCloser, error) {
			return &dying{body: []byte("partial"), err: exited}, nil
		}

		body, err := fixture.streamed(t, id)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("read %q then %v, want the truncation to reach the client as io.ErrUnexpectedEOF", body, err)
		}

		found := fixture.filed(t, id)
		if len(found) != 1 {
			t.Fatalf("got %d reports, want one", len(found))
		}
		if found[0].Origin != reports.OriginPlayback || found[0].Kind != reports.KindUnplayable || found[0].ItemSourceID == nil {
			t.Errorf("report = %+v", found[0])
		}
		if !strings.Contains(found[0].Note, "Error while decoding") {
			t.Errorf("note = %q, want what ffmpeg said", found[0].Note)
		}
	})

	t.Run("reports nothing when the relay fails for a reason other than ffmpeg", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.add(t, uuid.NewString()+".flac", "flac")

		fixture.transcoder.enabled = true
		fixture.transcoder.open = func(context.Context, transcode.Spec) (io.ReadCloser, error) {
			return &dying{body: []byte("partial"), err: errors.New("write: broken pipe")}, nil
		}

		if _, err := fixture.streamed(t, id); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("read error = %v, want io.ErrUnexpectedEOF", err)
		}
		if found := fixture.filed(t, id); len(found) != 0 {
			t.Errorf("got %d reports, want none", len(found))
		}
	})

	t.Run("reports nothing when the stall timer kills the encode", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.add(t, uuid.NewString()+".flac", "flac")

		fixture.transcoder.enabled = true
		fixture.transcoder.stall = 50 * time.Millisecond
		fixture.transcoder.open = func(ctx context.Context, _ transcode.Spec) (io.ReadCloser, error) {
			return &killed{ctx: ctx}, nil
		}

		if _, err := fixture.streamed(t, id); err == nil {
			t.Fatal("a killed encode ended the response cleanly")
		}
		if found := fixture.filed(t, id); len(found) != 0 {
			t.Errorf("got %d reports, want none", len(found))
		}
	})

	t.Run("reports nothing when the client goes away", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.add(t, uuid.NewString()+".flac", "flac")

		fixture.transcoder.enabled = true
		finished := make(chan struct{})
		fixture.transcoder.open = func(ctx context.Context, _ transcode.Spec) (io.ReadCloser, error) {
			return &killed{ctx: ctx}, nil
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(finished)
			r.SetPathValue("itemId", id.String())
			r.SetPathValue("container", "mp3")
			fixture.handler.Serve(w, r)
		}))
		t.Cleanup(server.Close)

		ctx, cancel := context.WithCancel(context.Background())
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/Audio/"+id.String()+"/stream.mp3?api_key="+fixture.token, nil)
		if err != nil {
			t.Fatalf("failed to build the request: %v", err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("failed to request the stream: %v", err)
		}
		cancel()
		_ = response.Body.Close()

		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Fatal("the handler did not notice the client leave")
		}
		if found := fixture.filed(t, id); len(found) != 0 {
			t.Errorf("got %d reports, want none", len(found))
		}
	})

	t.Run("ends the response cleanly when ffmpeg finishes", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.add(t, uuid.NewString()+".flac", "flac")

		fixture.transcoder.enabled = true
		fixture.transcoder.open = func(context.Context, transcode.Spec) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("complete")), nil
		}

		body, err := fixture.streamed(t, id)
		if err != nil || string(body) != "complete" {
			t.Fatalf("read %q, %v; want %q and a clean end", body, err, "complete")
		}
		if found := fixture.filed(t, id); len(found) != 0 {
			t.Errorf("got %d reports, want none", len(found))
		}
	})
}

func TestHandler_ReportsWhatWillNotPlay(t *testing.T) {
	t.Run("a source ffmpeg cannot read is reported once", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.add(t, uuid.NewString()+".flac", "flac")
		fixture.transcoder.enabled = true
		fixture.transcoder.open = func(context.Context, transcode.Spec) (io.ReadCloser, error) {
			return nil, errors.New("ffmpeg exited: exit status 1")
		}

		for range 2 {
			request := fixture.get(t, http.MethodGet, "/Audio/"+id.String()+"/stream.mp3?", id)
			request.SetPathValue("container", "mp3")
			recorder := httptest.NewRecorder()
			fixture.handler.Serve(recorder, request)

			if recorder.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
			}
		}

		found := fixture.filed(t, id)
		if len(found) != 1 {
			t.Fatalf("got %d reports, want one", len(found))
		}
		if found[0].Origin != reports.OriginPlayback || found[0].Kind != reports.KindUnplayable || found[0].ItemSourceID == nil {
			t.Errorf("report = %+v", found[0])
		}

		attempt := found[0].Attempt
		if attempt == nil {
			t.Fatal("the report records no attempt")
		}
		if attempt.UserID == nil || attempt.UserName == "" || attempt.Client != "Test" || attempt.ClientVersion != "1" || attempt.DeviceName != "Test" || attempt.Build == "" {
			t.Errorf("attempt = %+v, want the account, the device and the build that asked", attempt)
		}
		want := items.Playback{VideoDirect: true, Container: "mp3"}
		if served := attempt.Playback; served == nil || served.Direct != want.Direct || served.VideoDirect != want.VideoDirect ||
			served.AudioDirect != want.AudioDirect || served.Container != want.Container {
			t.Errorf("served = %+v, want %+v", served, want)
		}
	})

	t.Run("a file served whole that cannot be opened records a direct play", func(t *testing.T) {
		fixture := newFixture(t)
		item, source := fixture.scan(t, uuid.NewString()+".mp3")
		if err := os.Remove(source.Path); err != nil {
			t.Fatalf("failed to remove the file: %v", err)
		}

		request := fixture.get(t, http.MethodGet, "/Audio/"+item.ID.String()+"/stream?startTimeTicks=600000000&", item.ID)
		fixture.handler.Serve(httptest.NewRecorder(), request)

		found := fixture.filed(t, item.ID)
		if len(found) != 1 || found[0].Attempt == nil || found[0].Attempt.Playback == nil {
			t.Fatalf("reports = %+v, want one recording how it was served", found)
		}
		if attempt := found[0].Attempt; !attempt.Playback.Direct || attempt.StartTicks != 600000000 {
			t.Errorf("attempt = %+v, served = %+v, want a direct play from where the client asked", attempt, attempt.Playback)
		}
	})

	t.Run("a pod with every slot taken reports nothing", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.add(t, uuid.NewString()+".flac", "flac")
		fixture.transcoder.enabled = true
		fixture.transcoder.open = func(context.Context, transcode.Spec) (io.ReadCloser, error) {
			return nil, transcode.ErrBusy
		}

		request := fixture.get(t, http.MethodGet, "/Audio/"+id.String()+"/stream.mp3?", id)
		request.SetPathValue("container", "mp3")
		fixture.handler.Serve(httptest.NewRecorder(), request)

		if found := fixture.filed(t, id); len(found) != 0 {
			t.Errorf("got %d reports, want none", len(found))
		}
	})

	t.Run("a file that has gone from the disk is reported", func(t *testing.T) {
		fixture := newFixture(t)
		id := fixture.add(t, uuid.NewString()+".mp3", "mp3")
		paths, err := fixture.items.SourcePaths(context.Background(), id)
		if err != nil || len(paths) != 1 {
			t.Fatalf("paths = %v, %v", paths, err)
		}
		if err := os.Remove(paths[0]); err != nil {
			t.Fatalf("failed to remove the file: %v", err)
		}

		recorder := httptest.NewRecorder()
		fixture.handler.Serve(recorder, fixture.get(t, http.MethodGet, "/Audio/"+id.String()+"/stream?", id))

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
		if found := fixture.filed(t, id); len(found) != 1 {
			t.Errorf("got %d reports, want one", len(found))
		}
	})
}
