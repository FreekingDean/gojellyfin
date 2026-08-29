package artwork

import (
	"context"
	"io"
)

type Store interface {
	Put(ctx context.Context, key string, body io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, int64, bool, error)
	Delete(ctx context.Context, key string) error
}
