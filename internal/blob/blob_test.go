package blob

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/blob/blobtest"
	"github.com/FreekingDean/gojellyfin/internal/env"
)

func store(t *testing.T) *Store {
	t.Helper()

	built, err := newStore(blobtest.Server(t))
	if err != nil {
		t.Fatalf("failed to build the store: %v", err)
	}

	return built
}

func TestStore_RoundTrip(t *testing.T) {
	t.Run("reads back what it wrote", func(t *testing.T) {
		objects := store(t)
		body := []byte("poster-bytes")

		if err := objects.Put(context.Background(), "artwork/one/Primary/0", bytes.NewReader(body), int64(len(body)), "image/jpeg"); err != nil {
			t.Fatal(err)
		}

		found, err := objects.Get(context.Background(), "artwork/one/Primary/0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = found.Body.Close() }()

		read, err := io.ReadAll(found.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(read, body) {
			t.Errorf("got %q, want %q", read, body)
		}
		if found.ContentType != "image/jpeg" {
			t.Errorf("got content type %q, want image/jpeg", found.ContentType)
		}
		if found.Size != int64(len(body)) {
			t.Errorf("got size %d, want %d", found.Size, len(body))
		}
	})

	t.Run("misses a key nothing wrote", func(t *testing.T) {
		if _, err := store(t).Get(context.Background(), "artwork/nope/Primary/0"); !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("overwrites a key in place", func(t *testing.T) {
		objects := store(t)
		key := "artwork/one/Primary/0"

		if err := objects.Put(context.Background(), key, strings.NewReader("old"), 3, "image/jpeg"); err != nil {
			t.Fatal(err)
		}
		if err := objects.Put(context.Background(), key, strings.NewReader("new"), 3, "image/png"); err != nil {
			t.Fatal(err)
		}

		found, err := objects.Get(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = found.Body.Close() }()

		read, err := io.ReadAll(found.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(read) != "new" {
			t.Errorf("got %q, want new", read)
		}
	})

	t.Run("forgets a key it deleted", func(t *testing.T) {
		objects := store(t)
		key := "artwork/one/Primary/0"

		if err := objects.Put(context.Background(), key, strings.NewReader("bytes"), 5, "image/jpeg"); err != nil {
			t.Fatal(err)
		}
		if err := objects.Delete(context.Background(), key); err != nil {
			t.Fatal(err)
		}
		if _, err := objects.Get(context.Background(), key); !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})
}

func TestNewStore(t *testing.T) {
	t.Run("is disabled when no bucket is named", func(t *testing.T) {
		built, err := newStore(env.ObjectStore{})
		if err != nil {
			t.Fatal(err)
		}
		if built.Enabled() {
			t.Error("got an enabled store, want a disabled one")
		}
	})

	t.Run("answers a disabled store rather than writing nowhere", func(t *testing.T) {
		built, _ := newStore(env.ObjectStore{})

		if err := built.Put(context.Background(), "key", strings.NewReader("bytes"), 5, "image/jpeg"); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("got %v, want ErrNotConfigured", err)
		}
		if _, err := built.Get(context.Background(), "key"); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("got %v, want ErrNotConfigured", err)
		}
		if err := built.Delete(context.Background(), "key"); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("got %v, want ErrNotConfigured", err)
		}
	})

	t.Run("refuses a bucket with an endpoint it cannot read", func(t *testing.T) {
		for _, endpoint := range []string{"", "minio:9000", "://nope"} {
			if _, err := newStore(env.ObjectStore{Bucket: blobtest.Bucket, Endpoint: endpoint}); err == nil {
				t.Errorf("got no error for %q, want one", endpoint)
			}
		}
	})
}
