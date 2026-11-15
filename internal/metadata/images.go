package metadata

import (
	"context"
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

		if err := s.items.SaveImage(ctx, pendingItem.ID, reference.Image()); err != nil {
			log.Printf("metadata %s %s artwork: %v", pendingItem.Name, reference.Kind, err)
		}
	}
}
