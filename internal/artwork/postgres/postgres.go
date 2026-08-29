package postgres

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/FreekingDean/gojellyfin/internal/store"
	blobmodal "github.com/FreekingDean/gojellyfin/internal/store/imageblob"
)

type Client struct {
	store *store.Client
}

func New(client *store.Client) *Client {
	return &Client{store: client}
}

func (c *Client) Put(ctx context.Context, key string, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("failed to read the artwork for %q: %w", key, err)
	}

	err = c.store.ImageBlob.Create().
		SetKey(key).
		SetData(data).
		OnConflictColumns(blobmodal.FieldKey).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store the artwork for %q: %w", key, err)
	}

	return nil
}

func (c *Client) Open(ctx context.Context, key string) (io.ReadCloser, int64, bool, error) {
	blob, err := c.store.ImageBlob.Query().Where(blobmodal.Key(key)).Only(ctx)
	if store.IsNotFound(err) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, fmt.Errorf("failed to read the artwork for %q: %w", key, err)
	}

	return io.NopCloser(bytes.NewReader(blob.Data)), int64(len(blob.Data)), true, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if _, err := c.store.ImageBlob.Delete().Where(blobmodal.Key(key)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete the artwork for %q: %w", key, err)
	}

	return nil
}
