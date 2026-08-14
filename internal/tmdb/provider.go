package tmdb

import (
	"context"
	"errors"
	"log"
	"strconv"

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

type Provider struct {
	client *Client
	items  *items.Service
}

func New(client *Client, service *items.Service) *Provider {
	return &Provider{client: client, items: service}
}

// One step over the batch rather than a step per item: the work is IO bound on
// a rate limited API, so fanning out multiplies the request rate and finishes
// no sooner.
func (p *Provider) IdentifyItems(ctx context.Context) error {
	if !p.client.Enabled() {
		log.Print("tmdb: TMDB_API_KEY is not set, nothing to identify against")

		return nil
	}

	pending, err := p.items.UnidentifiedItems(ctx, identifiable, batchSize)
	if err != nil {
		return err
	}

	for _, pendingItem := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}

		jobs.Heartbeat(ctx, pendingItem.Name)

		if err := p.identify(ctx, pendingItem); err != nil {
			log.Printf("tmdb %s: %v", pendingItem.Name, err)
		}
	}

	return nil
}

// Nothing matching is not a failure: the title may be one TMDB gains later, and
// the next run asks again.
func (p *Provider) identify(ctx context.Context, pendingItem *items.Item) error {
	metadata, err := p.fetch(ctx, pendingItem)
	if errors.Is(err, errNoMatch) {
		return nil
	}
	if err != nil {
		return err
	}

	_, err = p.items.UpdateMetadata(ctx, pendingItem.ID, withoutLocked(metadata, pendingItem.LockedFields))

	return err
}

func (p *Provider) fetch(ctx context.Context, pendingItem *items.Item) (items.Metadata, error) {
	switch pendingItem.Kind {
	case itemmodal.KindMovie:
		return p.fetchMovie(ctx, pendingItem)
	case itemmodal.KindSeries:
		return p.fetchSeries(ctx, pendingItem)
	case itemmodal.KindEpisode:
		return p.fetchEpisode(ctx, pendingItem)
	}

	return items.Metadata{}, errNoMatch
}

func (p *Provider) fetchMovie(ctx context.Context, pendingItem *items.Item) (items.Metadata, error) {
	id, err := p.client.SearchMovie(ctx, pendingItem.Name, pendingItem.ProductionYear)
	if err != nil {
		return items.Metadata{}, err
	}

	movie, err := p.client.Movie(ctx, id)
	if err != nil {
		return items.Metadata{}, err
	}

	return movieMetadata(movie), nil
}

func (p *Provider) fetchSeries(ctx context.Context, pendingItem *items.Item) (items.Metadata, error) {
	id, err := p.client.SearchSeries(ctx, pendingItem.Name, pendingItem.ProductionYear)
	if err != nil {
		return items.Metadata{}, err
	}

	series, err := p.client.Series(ctx, id)
	if err != nil {
		return items.Metadata{}, err
	}

	return seriesMetadata(series), nil
}

// An episode is looked up under its series' id, so one whose series is not
// identified yet waits for the run that identifies it.
func (p *Provider) fetchEpisode(ctx context.Context, pendingItem *items.Item) (items.Metadata, error) {
	if pendingItem.IndexNumber == nil || pendingItem.ParentIndexNumber == nil {
		return items.Metadata{}, errNoMatch
	}

	seriesID, err := p.seriesID(ctx, pendingItem)
	if err != nil {
		return items.Metadata{}, err
	}

	episode, err := p.client.Episode(ctx, seriesID, *pendingItem.ParentIndexNumber, *pendingItem.IndexNumber)
	if err != nil {
		return items.Metadata{}, err
	}

	return episodeMetadata(episode), nil
}

func (p *Provider) seriesID(ctx context.Context, pendingItem *items.Item) (int, error) {
	ancestry, err := p.items.Ancestors(ctx, pendingItem.ID)
	if err != nil {
		return 0, err
	}
	if ancestry == nil {
		return 0, errNoMatch
	}

	for _, parent := range ancestry.Parents {
		if parent.Kind != itemmodal.KindSeries {
			continue
		}

		id, err := strconv.Atoi(parent.ProviderIds[providerTmdb])
		if err != nil {
			return 0, errNoMatch
		}

		return id, nil
	}

	return 0, errNoMatch
}
