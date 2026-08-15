package metadata

import (
	"context"
	"log"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

// Bounded so a run's history stays small and a crash re-derives what is
// outstanding from the rows rather than replaying a stale list.
const batchSize = 200

var identifiable = []items.Kind{
	itemmodal.KindMovie,
	itemmodal.KindSeries,
	itemmodal.KindEpisode,
}

type Service struct {
	provider Provider
	items    *items.Service
}

func New(provider Provider, service *items.Service) *Service {
	return &Service{provider: provider, items: service}
}

// One step over the batch rather than a step per item: the work is IO bound on
// a rate limited API, so fanning out multiplies the request rate and finishes
// no sooner.
func (s *Service) IdentifyItems(ctx context.Context) error {
	if !s.provider.Enabled() {
		log.Print("metadata: no provider is configured, nothing to identify against")

		return nil
	}

	pending, err := s.items.UnidentifiedItems(ctx, identifiable, batchSize)
	if err != nil {
		return err
	}

	for _, pendingItem := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}

		jobs.Heartbeat(ctx, pendingItem.Name)

		if err := s.identify(ctx, pendingItem); err != nil {
			log.Printf("metadata %s: %v", pendingItem.Name, err)
		}
	}

	return nil
}

// Nothing matching is not a failure: the title may be one a provider gains
// later, and the next run asks again.
func (s *Service) identify(ctx context.Context, pendingItem *items.Item) error {
	found, matched, err := s.fetch(ctx, pendingItem)
	if err != nil || !matched {
		return err
	}

	_, err = s.items.UpdateMetadata(ctx, pendingItem.ID, withoutLocked(found, pendingItem.LockedFields))

	return err
}

func (s *Service) fetch(ctx context.Context, pendingItem *items.Item) (items.Metadata, bool, error) {
	switch pendingItem.Kind {
	case itemmodal.KindMovie:
		return s.provider.Movie(ctx, pendingItem.Name, pendingItem.ProductionYear)
	case itemmodal.KindSeries:
		return s.provider.Series(ctx, pendingItem.Name, pendingItem.ProductionYear)
	case itemmodal.KindEpisode:
		return s.fetchEpisode(ctx, pendingItem)
	}

	return items.Metadata{}, false, nil
}

// An episode is looked up under its series' ids, so one whose series is not
// identified yet waits for the run that identifies it.
func (s *Service) fetchEpisode(ctx context.Context, pendingItem *items.Item) (items.Metadata, bool, error) {
	if pendingItem.IndexNumber == nil || pendingItem.ParentIndexNumber == nil {
		return items.Metadata{}, false, nil
	}

	series, err := s.seriesIDs(ctx, pendingItem)
	if err != nil || series == nil {
		return items.Metadata{}, false, err
	}

	return s.provider.Episode(ctx, series, *pendingItem.ParentIndexNumber, *pendingItem.IndexNumber)
}

func (s *Service) seriesIDs(ctx context.Context, pendingItem *items.Item) (map[string]string, error) {
	ancestry, err := s.items.Ancestors(ctx, pendingItem.ID)
	if err != nil || ancestry == nil {
		return nil, err
	}

	for _, parent := range ancestry.Parents {
		if parent.Kind == itemmodal.KindSeries {
			return parent.ProviderIds, nil
		}
	}

	return nil, nil
}
