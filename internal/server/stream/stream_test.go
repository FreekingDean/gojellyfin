package stream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var song = []byte("ID3 the quick brown fox jumps over the lazy dog")

type fixture struct {
	handler    *Handler
	items      *items.Service
	transcoder *stubTranscoder
	library    uuid.UUID
	token      string
}

// Off unless a test turns it on, which keeps direct play answering the way it
// does with no worker configured.
type stubTranscoder struct {
	enabled bool
	open    func(ctx context.Context, spec transcode.Spec) (io.ReadCloser, error)
	spec    transcode.Spec
}

func (s *stubTranscoder) Enabled() bool {
	return s.enabled
}

func (s *stubTranscoder) Open(ctx context.Context, spec transcode.Spec) (io.ReadCloser, error) {
	s.spec = spec

	return s.open(ctx, spec)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	client := connection.Client()
	unique := t.Name() + "-" + uuid.NewString()

	itemService := items.New(client)
	libraryService := libraries.New(client)
	sessionService := sessions.New(client)
	userService := users.New(client)

	library, err := libraryService.CreateLibrary(ctx, unique, librarymodal.CollectionTypeMusic, nil)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	user, err := userService.CreateUser(ctx, unique, "hash", true)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}

	token := uuid.NewString()
	device := sessions.DeviceInfo{ID: unique, Name: "Test", AppName: "Test", AppVersion: "1"}
	if _, err := sessionService.Create(ctx, user.ID, token, device); err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	t.Cleanup(func() {
		inLibrary := sourcemodal.HasItemWith(itemmodal.LibraryID(library.ID))
		if _, err := client.MediaStream.Delete().Where(streammodal.HasSourceWith(inLibrary)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media streams: %v", err)
		}
		if _, err := client.MediaSource.Delete().Where(inLibrary).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if err := libraryService.DeleteLibrary(ctx, library.ID); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := sessionService.DeleteByToken(ctx, token); err != nil {
			t.Errorf("failed to delete the session: %v", err)
		}
		if err := sessionService.RemoveDevice(ctx, unique); err != nil {
			t.Errorf("failed to delete the device: %v", err)
		}
		if err := userService.DeleteUser(ctx, user.ID); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	transcoder := &stubTranscoder{}

	return &fixture{
		handler:    New(sessionService, itemService, transcoder),
		items:      itemService,
		transcoder: transcoder,
		library:    library.ID,
		token:      token,
	}
}

func (f *fixture) scan(t *testing.T, name string) *items.Item {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, song, 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", path, err)
	}

	item, err := f.items.SaveScanned(context.Background(), items.Scanned{
		LibraryID:    f.library,
		Kind:         itemmodal.KindAudio,
		Name:         name,
		SortName:     name,
		Path:         path,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save %q: %v", name, err)
	}

	return item
}

func (f *fixture) add(t *testing.T, name, codec string) uuid.UUID {
	t.Helper()

	item := f.scan(t, name)
	err := f.items.SaveProbe(context.Background(), item, items.Probe{
		Container: strings.TrimPrefix(filepath.Ext(name), "."),
		Size:      int64(len(song)),
		Streams:   []items.Stream{{Kind: streammodal.KindAudio, Codec: codec}},
	})
	if err != nil {
		t.Fatalf("failed to probe %q: %v", name, err)
	}

	return item.ID
}

func (f *fixture) get(t *testing.T, method, target string, id uuid.UUID) *http.Request {
	t.Helper()

	request := httptest.NewRequest(method, target+"api_key="+f.token, nil)
	request.SetPathValue("itemId", id.String())

	return request
}

func TestServe(t *testing.T) {
	fixture := newFixture(t)
	id := fixture.add(t, "track.mp3", "mp3")
	target := "/Audio/" + id.String() + "/stream?"

	t.Run("serves the file", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		fixture.handler.Serve(recorder, fixture.get(t, http.MethodGet, target, id))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if got := recorder.Body.String(); got != string(song) {
			t.Errorf("body = %q, want %q", got, song)
		}
		if got := recorder.Header().Get("Content-Type"); got != "audio/mpeg" {
			t.Errorf("content type = %q, want audio/mpeg", got)
		}
		if got := recorder.Header().Get("Accept-Ranges"); got != "bytes" {
			t.Errorf("accept ranges = %q, want bytes", got)
		}
	})

	t.Run("answers a range request", func(t *testing.T) {
		request := fixture.get(t, http.MethodGet, target, id)
		request.Header.Set("Range", "bytes=4-8")
		recorder := httptest.NewRecorder()

		fixture.handler.Serve(recorder, request)

		if recorder.Code != http.StatusPartialContent {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusPartialContent)
		}
		if got := recorder.Body.String(); got != string(song[4:9]) {
			t.Errorf("body = %q, want %q", got, song[4:9])
		}
		want := fmt.Sprintf("bytes 4-8/%d", len(song))
		if got := recorder.Header().Get("Content-Range"); got != want {
			t.Errorf("content range = %q, want %q", got, want)
		}
	})

	t.Run("sends no body for a head request", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		fixture.handler.Serve(recorder, fixture.get(t, http.MethodHead, target, id))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if recorder.Body.Len() != 0 {
			t.Errorf("body = %q, want empty", recorder.Body.String())
		}
		if got := recorder.Header().Get("Content-Length"); got != strconv.Itoa(len(song)) {
			t.Errorf("content length = %q, want %d", got, len(song))
		}
	})

	t.Run("rejects an unauthenticated request", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.SetPathValue("itemId", id.String())
		recorder := httptest.NewRecorder()

		fixture.handler.Serve(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})
}

func TestServeByContainer(t *testing.T) {
	fixture := newFixture(t)
	id := fixture.add(t, "track.flac", "flac")

	for _, tc := range []struct {
		name      string
		container string
		query     string
		want      int
	}{
		{name: "the container the source is in", container: "flac", want: http.StatusOK},
		{name: "the same container in another case", container: "FLAC", want: http.StatusOK},
		{name: "a container needing a transcode", container: "mp3", want: http.StatusUnsupportedMediaType},
		{name: "a container overridden by static", container: "mp3", query: "static=true", want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := "/Audio/" + id.String() + "/stream." + tc.container + "?" + tc.query + "&"
			request := fixture.get(t, http.MethodGet, target, id)
			request.SetPathValue("container", tc.container)
			recorder := httptest.NewRecorder()

			fixture.handler.Serve(recorder, request)

			if recorder.Code != tc.want {
				t.Errorf("status = %d, want %d", recorder.Code, tc.want)
			}
		})
	}
}

func TestServeUniversal(t *testing.T) {
	fixture := newFixture(t)
	mp3 := fixture.add(t, "track.mp3", "mp3")
	alac := fixture.add(t, "lossless.m4a", "alac")
	unprobed := fixture.scan(t, "unprobed.mp3").ID

	for _, tc := range []struct {
		name      string
		item      uuid.UUID
		container string
		want      int
	}{
		{name: "no declared containers", item: mp3, want: http.StatusOK},
		{name: "a declared container", item: mp3, container: "opus,mp3,flac", want: http.StatusOK},
		{name: "a declared container and codec", item: mp3, container: "webm|opus,mp3|mp3", want: http.StatusOK},
		{name: "a container declaring several codecs", item: alac, container: "m4a|aac|alac", want: http.StatusOK},
		{name: "a container the source is not in", item: mp3, container: "opus,flac", want: http.StatusUnsupportedMediaType},
		{name: "the container but not the codec", item: alac, container: "m4a|aac", want: http.StatusUnsupportedMediaType},
		{name: "an unprobed source with an unknown codec", item: unprobed, container: "mp3|mp3", want: http.StatusOK},
		{name: "an unprobed source in another container", item: unprobed, container: "flac|flac", want: http.StatusUnsupportedMediaType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := "/Audio/" + tc.item.String() + "/universal?container=" + tc.container + "&"
			recorder := httptest.NewRecorder()

			fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodGet, target, tc.item))

			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.want)
			}
			if tc.want != http.StatusOK {
				return
			}
			if got := recorder.Body.String(); got != string(song) {
				t.Errorf("body = %q, want %q", got, song)
			}
		})
	}

	t.Run("sends no body for a head request", func(t *testing.T) {
		target := "/Audio/" + mp3.String() + "/universal?container=mp3&"
		recorder := httptest.NewRecorder()

		fixture.handler.ServeUniversal(recorder, fixture.get(t, http.MethodHead, target, mp3))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if recorder.Body.Len() != 0 {
			t.Errorf("body = %q, want empty", recorder.Body.String())
		}
		if got := recorder.Header().Get("Content-Length"); got != strconv.Itoa(len(song)) {
			t.Errorf("content length = %q, want %d", got, len(song))
		}
	})

	t.Run("rejects an unauthenticated request", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/Audio/"+mp3.String()+"/universal", nil)
		request.SetPathValue("itemId", mp3.String())
		recorder := httptest.NewRecorder()

		fixture.handler.ServeUniversal(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})
}
