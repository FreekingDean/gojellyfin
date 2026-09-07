package metadata

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"log"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
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

func tag(url string) string {
	sum := sha1.Sum([]byte(url))

	return hex.EncodeToString(sum[:])
}
