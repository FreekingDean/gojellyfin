package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

var (
	ErrNotConfigured = errors.New("blob: OBJECT_STORE_BUCKET is not set")
	ErrNotFound      = errors.New("blob: the object is not there")
)

type Store struct {
	client *minio.Client
	bucket string
}

type Object struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

func New(config env.Config) (*Store, error) {
	return newStore(config.ObjectStore)
}

func newStore(config env.ObjectStore) (*Store, error) {
	if config.Bucket == "" {
		return &Store{}, nil
	}

	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, fmt.Errorf("OBJECT_STORE_ENDPOINT must be an http or https url naming the object store, got %q", config.Endpoint)
	}

	client, err := minio.New(endpoint.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: endpoint.Scheme == "https",
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("the object store could not be built: %w", err)
	}

	return &Store{client: client, bucket: config.Bucket}, nil
}

func (s *Store) Enabled() bool {
	return s.client != nil
}

func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}

	if _, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("failed to write %s: %w", key, err)
	}

	return nil
}

func (s *Store) Get(ctx context.Context, key string) (Object, error) {
	if !s.Enabled() {
		return Object{}, ErrNotConfigured
	}

	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return Object{}, fmt.Errorf("failed to read %s: %w", key, err)
	}

	info, err := object.Stat()
	if err != nil {
		_ = object.Close()

		if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
			return Object{}, ErrNotFound
		}

		return Object{}, fmt.Errorf("failed to read %s: %w", key, err)
	}

	return Object{Body: object, ContentType: info.ContentType, Size: info.Size}, nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}

	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("failed to delete %s: %w", key, err)
	}

	return nil
}
