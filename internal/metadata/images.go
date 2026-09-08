package metadata

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

const (
	artworkBatchSize = 500
	artworkTimeout   = 30 * time.Second
	artworkMediaType = "application/octet-stream"
)

func (s *Service) saveArtwork(ctx context.Context, pendingItem *items.Item, references []items.RemoteImage) {
	for _, reference := range references {
		if err := ctx.Err(); err != nil {
			return
		}

		jobs.Heartbeat(ctx, pendingItem.Name, string(reference.Kind))

		image := items.Image{Kind: reference.Kind, URL: reference.URL, Tag: tag(reference.URL)}
		if err := s.items.SaveImage(ctx, pendingItem.ID, image); err != nil {
			log.Printf("metadata %s %s artwork: %v", pendingItem.Name, reference.Kind, err)
		}
	}
}

func (s *Service) CacheArtwork(ctx context.Context, scope uuid.UUID) error {
	if !s.blob.Enabled() {
		log.Print("metadata: no object store is configured, artwork stays on the provider's cdn")

		return nil
	}

	pending, err := s.items.ImagesNeedingCache(ctx, scope, artworkBatchSize)
	if err != nil {
		return err
	}

	for _, image := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}

		jobs.Heartbeat(ctx, image.ItemID, string(image.Kind))

		if err := s.cache(ctx, image); err != nil {
			log.Printf("metadata artwork %s %s: %v", image.ItemID, image.Kind, err)
		}
	}

	return nil
}

func (s *Service) cache(ctx context.Context, image *items.Image) error {
	ctx, cancel := context.WithTimeout(ctx, artworkTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to build the artwork request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", image.URL, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch %s: %s", image.URL, response.Status)
	}

	key := items.ImageKey(image.ItemID, image.Kind, image.Index)
	if err := s.blob.Put(ctx, key, response.Body, response.ContentLength, mediaType(response)); err != nil {
		return err
	}

	return s.items.SaveImageKey(ctx, image.ID, key)
}

func mediaType(response *http.Response) string {
	if found := response.Header.Get("Content-Type"); found != "" {
		return found
	}

	return artworkMediaType
}

func tag(url string) string {
	sum := sha1.Sum([]byte(url))

	return hex.EncodeToString(sum[:])
}
