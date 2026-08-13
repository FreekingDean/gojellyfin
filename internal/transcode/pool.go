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

var ErrNoWorker = errors.New("no transcoder answered")

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

// Round robin rather than least busy: a transcode is one request from start to
// finish, so there is nothing to keep together and nothing worth a load query
// to every worker before each one. Each worker in turn is tried, which covers
// a pod that is restarting as long as it has not written any output yet.
func (p *Pool) Open(ctx context.Context, spec Spec) (io.ReadCloser, error) {
	if !p.Enabled() {
		return nil, ErrNoWorker
	}
	if err := spec.Valid(); err != nil {
		return nil, err
	}

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

		log.Printf("transcoder %s refused %s: %v", worker, spec.Path, err)
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

		return nil, fmt.Errorf("transcoder answered %s", response.Status)
	}

	return response.Body, nil
}
