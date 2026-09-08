package metadata

import (
	"context"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

var identifiable = []items.Kind{
	itemmodal.KindMovie,
	itemmodal.KindSeries,
	itemmodal.KindSeason,
	itemmodal.KindEpisode,
}

type Service struct {
	provider Provider
	items    *items.Service
}

func New(provider Provider, service *items.Service) *Service {
	return &Service{provider: provider, items: service}
}

func (s *Service) IdentifyItem(ctx context.Context, id uuid.UUID) error {
	if !s.provider.Enabled() {
		return nil
	}

	pendingItem, err := s.items.ItemByID(ctx, items.Everyone, id)
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	jobs.Heartbeat(ctx, pendingItem.Name)

	return s.identify(ctx, pendingItem)
}

func (s *Service) IdentifyItems(ctx context.Context, scope uuid.UUID, force bool) error {
	if !s.provider.Enabled() {
		log.Print("metadata: no provider is configured, nothing to identify against")

		return nil
	}

	pending, err := s.items.ItemsNeedingMetadata(ctx, identifiable, force, scope)
	if err != nil {
		return err
	}

	for _, id := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}

		jobs.Heartbeat(ctx, id)

		pendingItem, err := s.items.ItemByID(ctx, items.Everyone, id)
		if store.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}

		jobs.Heartbeat(ctx, pendingItem.Name)

		if err := s.identify(ctx, pendingItem); err != nil {
			log.Printf("metadata %s: %v", pendingItem.Name, err)
		}
	}

	return nil
}

func (s *Service) identify(ctx context.Context, pendingItem *items.Item) error {
	found, matched, err := s.fetch(ctx, pendingItem)
	if err != nil || !matched {
		return err
	}

	stripLockedFields(&found, pendingItem.LockedFields)

	if _, err := s.items.UpdateMetadata(ctx, pendingItem.ID, found); err != nil {
		return err
	}

	s.saveArtwork(ctx, pendingItem, found.Images)

	return nil
}

func (s *Service) fetch(ctx context.Context, pendingItem *items.Item) (items.Metadata, bool, error) {
	tmdbID, ok := items.TmdbID(pendingItem.Key)
	if !ok {
		return items.Metadata{}, false, nil
	}

	switch pendingItem.Kind {
	case itemmodal.KindMovie:
		return s.provider.Movie(ctx, tmdbID)
	case itemmodal.KindSeries:
		return s.provider.Series(ctx, tmdbID)
	case itemmodal.KindSeason:
		if pendingItem.IndexNumber == nil {
			return items.Metadata{}, false, nil
		}

		return s.provider.Season(ctx, tmdbID, *pendingItem.IndexNumber)
	case itemmodal.KindEpisode:
		if pendingItem.IndexNumber == nil || pendingItem.ParentIndexNumber == nil {
			return items.Metadata{}, false, nil
		}

		return s.provider.Episode(ctx, tmdbID, *pendingItem.ParentIndexNumber, *pendingItem.IndexNumber)
	}

	return items.Metadata{}, false, nil
}
