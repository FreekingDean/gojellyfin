package metadata

import (
	"context"
	"log"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

// Parents before children: a season resolves its series through the series'
// provider ids and an episode through the season's, so one identified after its
// child leaves that child a miss until the next run. The query hands the batch
// back in this order.
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

// The whole outstanding list rather than a page of it: the provider is rate
// limited to one request at a time inside this process, so chunking wins no
// parallelism and a cap only means pressing Start again.
func (s *Service) IdentifyItems(ctx context.Context, options jobs.Options) error {
	if !s.provider.Enabled() {
		log.Print("metadata: no provider is configured, nothing to identify against")

		return nil
	}

	pending, err := s.items.ItemsNeedingMetadata(ctx, identifiable, options.Force, options.Scope)
	if err != nil {
		return err
	}

	for _, id := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}

		jobs.Heartbeat(ctx, id)

		pendingItem, err := s.items.ItemByID(ctx, id)
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

	_, err = s.items.UpdateMetadata(ctx, pendingItem.ID, found)

	return err
}

func (s *Service) fetch(ctx context.Context, pendingItem *items.Item) (items.Metadata, bool, error) {
	switch pendingItem.Kind {
	case itemmodal.KindMovie:
		return s.provider.Movie(ctx, pendingItem.Name, pendingItem.ProductionYear)
	case itemmodal.KindSeries:
		return s.provider.Series(ctx, pendingItem.Name, pendingItem.ProductionYear)
	case itemmodal.KindSeason:
		return s.fetchSeason(ctx, pendingItem)
	case itemmodal.KindEpisode:
		return s.fetchEpisode(ctx, pendingItem)
	}

	return items.Metadata{}, false, nil
}

func (s *Service) fetchSeason(ctx context.Context, pendingItem *items.Item) (items.Metadata, bool, error) {
	if pendingItem.IndexNumber == nil {
		return items.Metadata{}, false, nil
	}

	series, err := s.seriesIDs(ctx, pendingItem)
	if err != nil || series == nil {
		return items.Metadata{}, false, err
	}

	return s.provider.Season(ctx, series, *pendingItem.IndexNumber)
}

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
