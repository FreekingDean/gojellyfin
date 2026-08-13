package worker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

// Far more output than the socket buffers between the two ends can hold, so a
// worker is left blocked in its write rather than finishing on its own, which
// is what holds a job open for as long as a test needs it.
const (
	streamSeconds = 300
	streamBitrate = 320_000
)

func newWorker(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	if !Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	requests := &atomic.Int64{}
	handler := New().Handler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	return server, requests
}

// The body is closed on cleanup rather than here: a request nothing reads holds
// its job for as long as the test needs, and httptest waits for it on Close.
func request(t *testing.T, server *httptest.Server, path string) *http.Response {
	t.Helper()

	spec := transcode.Spec{Path: path, Container: "mp3", Bitrate: streamBitrate}
	response, err := http.Get(server.URL + transcode.Path + "?" + spec.Query().Encode())
	if err != nil {
		t.Fatalf("failed to reach the worker: %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })

	return response
}

func TestHandleRefusesWhenEveryJobIsTaken(t *testing.T) {
	t.Setenv("TRANSCODER_JOBS", "1")
	server, _ := newWorker(t)
	tone := source(t, "tone.flac", streamSeconds)

	if status := request(t, server, tone).StatusCode; status != http.StatusOK {
		t.Fatalf("the first transcode answered %d, want %d", status, http.StatusOK)
	}
	if status := request(t, server, tone).StatusCode; status != http.StatusServiceUnavailable {
		t.Fatalf("the second transcode answered %d, want %d", status, http.StatusServiceUnavailable)
	}
}

// The refusal is the whole mechanism: the pool reads it as "try the next one",
// so a busy worker is skipped and an idle one does the work.
func TestPoolSkipsAWorkerAtItsLimit(t *testing.T) {
	t.Setenv("TRANSCODER_JOBS", "1")
	full, fullRequests := newWorker(t)
	idle, idleRequests := newWorker(t)
	tone := source(t, "tone.flac", streamSeconds)

	if status := request(t, full, tone).StatusCode; status != http.StatusOK {
		t.Fatalf("the job that fills the worker answered %d, want %d", status, http.StatusOK)
	}

	t.Setenv("TRANSCODER_WORKERS", full.URL+","+idle.URL)
	pool := transcode.NewPool()

	output, err := pool.Open(context.Background(), transcode.Spec{
		Path:      tone,
		Container: "mp3",
		Bitrate:   streamBitrate,
	})
	if err != nil {
		t.Fatalf("the pool refused while a worker was idle: %v", err)
	}
	defer func() { _ = output.Close() }()

	if got := fullRequests.Load(); got != 2 {
		t.Errorf("the full worker saw %d requests, want the one that filled it and one it refused", got)
	}
	if got := idleRequests.Load(); got != 1 {
		t.Errorf("the idle worker saw %d requests, want 1", got)
	}

	body := make([]byte, 128*1024)
	if _, err := io.ReadFull(output, body); err != nil {
		t.Fatalf("failed to read the stream the idle worker served: %v", err)
	}
	if probed := probe(t, body, "out.mp3"); len(probed.Streams) == 0 || probed.Streams[0].CodecName != "mp3" {
		t.Fatalf("the idle worker did not serve mp3: %+v", probed.Streams)
	}
}

func TestPoolReportsBusyWhenEveryWorkerIsFull(t *testing.T) {
	t.Setenv("TRANSCODER_JOBS", "1")
	first, _ := newWorker(t)
	second, _ := newWorker(t)
	tone := source(t, "tone.flac", streamSeconds)

	for _, server := range []*httptest.Server{first, second} {
		if status := request(t, server, tone).StatusCode; status != http.StatusOK {
			t.Fatalf("the job that fills a worker answered %d, want %d", status, http.StatusOK)
		}
	}

	t.Setenv("TRANSCODER_WORKERS", first.URL+","+second.URL)
	pool := transcode.NewPool()

	answered := make(chan error, 1)
	go func() {
		output, err := pool.Open(context.Background(), transcode.Spec{
			Path:      tone,
			Container: "mp3",
			Bitrate:   streamBitrate,
		})
		if output != nil {
			_ = output.Close()
		}

		answered <- err
	}()

	select {
	case err := <-answered:
		if !errors.Is(err, transcode.ErrBusy) {
			t.Fatalf("err = %v, want %v", err, transcode.ErrBusy)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the pool never answered with every worker full")
	}
}
