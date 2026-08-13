package worker

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func encoders(t *testing.T, path string) int {
	t.Helper()

	output, err := exec.Command("pgrep", "-f", "ffmpeg.*"+path).Output()
	if err != nil {
		exit := &exec.ExitError{}
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return 0
		}

		t.Fatalf("failed to count the encoders of %s: %v", path, err)
	}

	return len(strings.Fields(string(output)))
}

// A client that goes away without closing leaves the worker blocked in a write
// that TCP will not fail for hours, with ffmpeg parked on a full pipe behind it.
func TestHandleStopsATranscodeNothingIsReading(t *testing.T) {
	t.Setenv("TRANSCODER_STALL_TIMEOUT", "1s")
	server, _ := newWorker(t)
	tone := source(t, "tone.flac", streamSeconds)

	if count := encoders(t, tone); count != 0 {
		t.Fatalf("%d encoders of %s before the request", count, tone)
	}

	response := request(t, server, tone)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the transcode answered %d, want %d", response.StatusCode, http.StatusOK)
	}
	if _, err := io.ReadFull(response.Body, make([]byte, 1)); err != nil {
		t.Fatalf("failed to read the stream: %v", err)
	}
	if count := encoders(t, tone); count != 1 {
		t.Fatalf("%d encoders of %s while streaming, want 1", count, tone)
	}

	deadline := time.Now().Add(10 * time.Second)
	for encoders(t, tone) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the encode outlived the client that stopped reading it")
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// The regression that would break playback on a poor connection: a slow client
// keeps the pipe full for far longer than the stall timeout, and every byte it
// takes is proof it is still there.
func TestHandleKeepsASlowReaderStreaming(t *testing.T) {
	t.Setenv("TRANSCODER_STALL_TIMEOUT", "1s")
	server, _ := newWorker(t)
	tone := source(t, "tone.flac", streamSeconds)

	response := request(t, server, tone)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the transcode answered %d, want %d", response.StatusCode, http.StatusOK)
	}

	// Enough to outlast the buffers on the client's side of the connection,
	// which would otherwise keep answering reads long after the worker stopped.
	body := &bytes.Buffer{}
	chunk := make([]byte, 256*1024)
	for range 20 {
		time.Sleep(200 * time.Millisecond)

		if _, err := io.ReadFull(response.Body, chunk); err != nil {
			t.Fatalf("the stream ended after %d bytes read a chunk at a time: %v", body.Len(), err)
		}

		body.Write(chunk)
	}

	if probed := probe(t, body.Bytes(), "out.mp3"); len(probed.Streams) == 0 || probed.Streams[0].CodecName != "mp3" {
		t.Fatalf("the slow reader did not get mp3: %+v", probed.Streams)
	}
}
