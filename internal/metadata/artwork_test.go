package metadata

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const onePixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

type cdn struct {
	server *httptest.Server

	mutex  sync.Mutex
	bodies map[string][]byte
	asked  []string
}

func newCDN(t *testing.T) *cdn {
	t.Helper()

	served := &cdn{bodies: map[string][]byte{}}
	served.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		served.mutex.Lock()
		served.asked = append(served.asked, request.URL.Path)
		body, found := served.bodies[request.URL.Path]
		served.mutex.Unlock()

		if !found {
			writer.WriteHeader(http.StatusNotFound)

			return
		}

		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(body)
	}))
	t.Cleanup(served.server.Close)

	return served
}

func (c *cdn) serve(t *testing.T, path string) (string, []byte) {
	t.Helper()

	body, err := base64.StdEncoding.DecodeString(onePixel)
	if err != nil {
		t.Fatalf("failed to decode the image: %v", err)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.bodies[path] = body

	return c.server.URL + path, body
}

func (c *cdn) requests() []string {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return append([]string(nil), c.asked...)
}

func (f *fixture) poster(t *testing.T, id uuid.UUID) *items.Image {
	t.Helper()

	record, err := f.items.Image(context.Background(), id, imagemodal.KindPrimary, 0)
	if err != nil {
		t.Fatalf("failed to read the poster: %v", err)
	}

	return record
}

func (f *fixture) stored(t *testing.T, key string) []byte {
	t.Helper()

	body, _, found, err := f.artwork.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("failed to open %q: %v", key, err)
	}
	if !found {
		t.Fatalf("nothing is stored under %q", key)
	}
	defer func() { _ = body.Close() }()

	read, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read %q: %v", key, err)
	}

	return read
}

func (f *fixture) matrix(t *testing.T) *items.Item {
	t.Helper()

	return f.add(t, items.Scanned{
		Kind:           itemmodal.KindMovie,
		Name:           "The Matrix",
		ProductionYear: index(1999),
	})
}

func TestService_IdentifyItems_Artwork(t *testing.T) {
	t.Run("downloads the poster the provider names", func(t *testing.T) {
		fixed := newFixture(t)
		served := newCDN(t)
		url, body := served.serve(t, "/t/p/w780/matrix.png")
		fixed.provider.images = []items.RemoteImage{{Kind: imagemodal.KindPrimary, URL: url}}

		movie := fixed.matrix(t)
		fixed.identify(t)

		record := fixed.poster(t, movie.ID)
		if record.Size != int64(len(body)) {
			t.Errorf("size = %d, want %d", record.Size, len(body))
		}
		if got := fixed.stored(t, record.Path); string(got) != string(body) {
			t.Errorf("stored %d bytes, want the %d the CDN served", len(got), len(body))
		}
	})

	t.Run("downloads a poster it already holds only once", func(t *testing.T) {
		fixed := newFixture(t)
		served := newCDN(t)
		url, _ := served.serve(t, "/t/p/w780/matrix.png")
		fixed.provider.images = []items.RemoteImage{{Kind: imagemodal.KindPrimary, URL: url}}

		fixed.matrix(t)
		fixed.identify(t)
		fixed.run(t, jobs.Options{Force: true})

		if asked := served.requests(); len(asked) != 1 {
			t.Errorf("requests = %v, want the poster fetched once", asked)
		}
	})

	t.Run("replaces a poster the provider has changed", func(t *testing.T) {
		fixed := newFixture(t)
		served := newCDN(t)
		first, _ := served.serve(t, "/t/p/w780/matrix.png")
		fixed.provider.images = []items.RemoteImage{{Kind: imagemodal.KindPrimary, URL: first}}

		movie := fixed.matrix(t)
		fixed.identify(t)
		was := fixed.poster(t, movie.ID)

		second, body := served.serve(t, "/t/p/w780/matrix-remastered.png")
		fixed.provider.images = []items.RemoteImage{{Kind: imagemodal.KindPrimary, URL: second}}
		fixed.run(t, jobs.Options{Force: true})

		record := fixed.poster(t, movie.ID)
		if record.Path == was.Path {
			t.Errorf("path = %q, want the changed poster stored under its own key", record.Path)
		}
		if record.Tag == was.Tag {
			t.Errorf("tag = %q, want a changed poster to bust the client's cache", record.Tag)
		}
		if got := fixed.stored(t, record.Path); string(got) != string(body) {
			t.Errorf("stored %d bytes, want the %d the CDN served", len(got), len(body))
		}
		if _, _, found, err := fixed.artwork.Open(context.Background(), was.Path); err != nil || found {
			t.Errorf("the replaced poster is still stored under %q", was.Path)
		}
	})

	t.Run("carries on past artwork it cannot fetch", func(t *testing.T) {
		fixed := newFixture(t)
		served := newCDN(t)
		backdrop, body := served.serve(t, "/t/p/w1280/matrix.png")
		fixed.provider.images = []items.RemoteImage{
			{Kind: imagemodal.KindPrimary, URL: served.server.URL + "/t/p/w780/missing.png"},
			{Kind: imagemodal.KindBackdrop, URL: backdrop},
		}

		movie := fixed.matrix(t)
		fixed.identify(t)

		if identified := fixed.reload(t, movie.ID); identified.ProviderIds["Stub"] != "603" {
			t.Errorf("provider id = %q, want the metadata written anyway", identified.ProviderIds["Stub"])
		}
		if _, err := fixed.items.Image(context.Background(), movie.ID, imagemodal.KindPrimary, 0); err == nil {
			t.Error("a poster the CDN refused was written as a row")
		}

		record, err := fixed.items.Image(context.Background(), movie.ID, imagemodal.KindBackdrop, 0)
		if err != nil {
			t.Fatalf("the run stopped at the missing poster: %v", err)
		}
		if got := fixed.stored(t, record.Path); string(got) != string(body) {
			t.Errorf("stored %d bytes, want the %d the CDN served", len(got), len(body))
		}
	})
}
