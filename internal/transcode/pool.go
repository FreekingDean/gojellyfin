package transcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// ffmpeg has to open the source and fill the muxer before it writes a byte,
// and the worker holds the response headers back until it does, so this bounds
// startup rather than the stream, which runs as long as the client listens.
const startTimeout = 30 * time.Second

var (
	ErrNoWorker = errors.New("no transcoder answered")
	ErrBusy     = errors.New("every transcoder is busy")
)

type Pool struct {
	workers []string
	token   string
	client  *http.Client
	next    atomic.Uint64
}

func NewPool() *Pool {
	return &Pool{
		workers: addresses(os.Getenv("TRANSCODER_WORKERS")),
		token:   os.Getenv("TRANSCODER_TOKEN"),
		client: &http.Client{
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
				ResponseHeaderTimeout: startTimeout,
			},
		},
	}
}

func addresses(value string) []string {
	workers := make([]string, 0)
	for _, address := range strings.Split(value, ",") {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		if !strings.Contains(address, "://") {
			address = "http://" + address
		}

		workers = append(workers, strings.TrimSuffix(address, "/"))
	}

	return workers
}

func (p *Pool) Enabled() bool {
	return len(p.workers) > 0
}

// Round robin, offset by a counter this process owns alone. A worker at its
// job limit refuses with a 503 and the next one is tried, which is what makes
// the spread least loaded rather than lucky without a load query to every
// worker first, and without state a second API replica would have to share. A
// pod that is restarting is skipped the same way, as long as it has not
// written any output yet.
func (p *Pool) Open(ctx context.Context, spec Spec) (io.ReadCloser, error) {
	if !p.Enabled() {
		return nil, ErrNoWorker
	}
	if err := spec.Valid(); err != nil {
		return nil, err
	}

	busy := false
	start := int(p.next.Add(1) - 1)
	for offset := range p.workers {
		worker := p.workers[(start+offset)%len(p.workers)]

		body, err := p.open(ctx, worker, spec)
		if err == nil {
			return body, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		busy = busy || errors.Is(err, ErrBusy)

		log.Printf("transcoder %s refused %s: %v", worker, spec.Path, err)
	}

	// One worker that is merely full is enough to make this worth retrying,
	// however many of the others are down.
	if busy {
		return nil, ErrBusy
	}

	return nil, ErrNoWorker
}

func (p *Pool) open(ctx context.Context, worker string, spec Spec) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, worker+Path+"?"+spec.Query().Encode(), nil)
	if err != nil {
		return nil, err
	}
	if p.token != "" {
		request.Header.Set(TokenHeader, p.token)
	}

	response, err := p.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		if response.StatusCode == http.StatusServiceUnavailable {
			return nil, ErrBusy
		}

		return nil, fmt.Errorf("transcoder answered %s", response.Status)
	}

	return response.Body, nil
}
