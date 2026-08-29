package artwork

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

func newStore(t *testing.T) Store {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	t.Cleanup(func() {
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return New(connection.Client())
}

func read(t *testing.T, artwork Store, key string) ([]byte, int64, bool) {
	t.Helper()

	body, size, found, err := artwork.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("failed to open %q: %v", key, err)
	}
	if !found {
		return nil, 0, false
	}

	defer func() { _ = body.Close() }()

	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read %q: %v", key, err)
	}

	return content, size, true
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	artwork := newStore(t)
	key := t.Name() + "/poster.jpg"
	t.Cleanup(func() {
		if err := artwork.Delete(ctx, key); err != nil {
			t.Errorf("failed to clean up %q: %v", key, err)
		}
	})

	t.Run("misses a key nothing put", func(t *testing.T) {
		if _, _, found := read(t, artwork, key); found {
			t.Errorf("found %q, want a miss", key)
		}
	})

	t.Run("reads back what it stored", func(t *testing.T) {
		poster := bytes.Repeat([]byte("poster"), 4096)
		if err := artwork.Put(ctx, key, bytes.NewReader(poster)); err != nil {
			t.Fatalf("failed to put %q: %v", key, err)
		}

		content, size, found := read(t, artwork, key)
		if !found {
			t.Fatalf("missed %q after putting it", key)
		}
		if !bytes.Equal(content, poster) {
			t.Errorf("content = %d bytes, want the %d stored", len(content), len(poster))
		}
		if size != int64(len(poster)) {
			t.Errorf("size = %d, want %d", size, len(poster))
		}
	})

	t.Run("replaces a key that already holds bytes", func(t *testing.T) {
		replacement := []byte("a smaller poster")
		if err := artwork.Put(ctx, key, bytes.NewReader(replacement)); err != nil {
			t.Fatalf("failed to replace %q: %v", key, err)
		}

		content, size, found := read(t, artwork, key)
		if !found {
			t.Fatalf("missed %q after replacing it", key)
		}
		if !bytes.Equal(content, replacement) {
			t.Errorf("content = %q, want %q", content, replacement)
		}
		if size != int64(len(replacement)) {
			t.Errorf("size = %d, want %d", size, len(replacement))
		}
	})

	t.Run("deletes", func(t *testing.T) {
		if err := artwork.Delete(ctx, key); err != nil {
			t.Fatalf("failed to delete %q: %v", key, err)
		}
		if _, _, found := read(t, artwork, key); found {
			t.Errorf("found %q after deleting it", key)
		}
	})

	t.Run("deletes a key nothing put", func(t *testing.T) {
		if err := artwork.Delete(ctx, t.Name()+"/absent.jpg"); err != nil {
			t.Errorf("failed to delete an absent key: %v", err)
		}
	})
}
