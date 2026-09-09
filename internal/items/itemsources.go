package items

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
)

type (
	MediaSource = store.ItemSourceModel
	MediaStream = store.MediaStreamModel
	StreamKind  = streammodel.Kind

	VideoRangeType = streammodel.VideoRangeType
)

func (s *Service) SaveSource(ctx context.Context, scanned MediaSource) (*MediaSource, error) {
	id, err := s.store.ItemSource.Create().
		SetSourceID(scanned.SourceID).
		SetItemID(scanned.ItemID).
		SetPath(scanned.Path).
		SetName(scanned.Name).
		SetNillableDateModified(scanned.DateModified).
		OnConflictColumns(sourcemodel.FieldPath).
		UpdateItemID().
		UpdateSourceID().
		UpdateName().
		UpdateDateModified().
		UpdateUpdatedAt().
		ID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to save media source: %w", err)
	}

	source, err := s.store.ItemSource.GetModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query the saved media source: %w", err)
	}

	return source, nil
}

func (s *Service) SaveProbe(ctx context.Context, item *Item, source *MediaSource, probe MediaSource) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		err := tx.ItemSource.UpdateOneID(source.ID).
			SetContainer(probe.Container).
			SetNillableRunTimeTicks(probe.RunTimeTicks).
			SetNillableSize(probe.Size).
			SetNillableBitrate(probe.Bitrate).
			SetProbedAt(time.Now()).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to save the probed media source: %w", err)
		}

		if _, err := tx.MediaStream.Delete().Where(streammodel.ItemSourceID(source.ID)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to clear media streams: %w", err)
		}

		if len(probe.Streams) == 0 {
			return nil
		}

		builders := make([]*store.MediaStreamCreate, 0, len(probe.Streams))
		for _, stream := range probe.Streams {
			builders = append(builders, tx.MediaStream.Create().
				SetSourceID(source.ID).
				SetIndex(stream.Index).
				SetKind(stream.Kind).
				SetCodec(stream.Codec).
				SetProfile(stream.Profile).
				SetLanguage(stream.Language).
				SetTitle(stream.Title).
				SetNillableWidth(stream.Width).
				SetNillableHeight(stream.Height).
				SetNillableChannels(stream.Channels).
				SetNillableSampleRate(stream.SampleRate).
				SetNillableBitRate(stream.BitRate).
				SetPixelFormat(stream.PixelFormat).
				SetNillableLevel(stream.Level).
				SetIsDefault(stream.IsDefault).
				SetIsForced(stream.IsForced).
				SetNillableVideoRangeType(stream.VideoRangeType).
				SetIsInterlaced(stream.IsInterlaced).
				SetIsAnamorphic(stream.IsAnamorphic))
		}
		if err := tx.MediaStream.CreateBulk(builders...).Exec(ctx); err != nil {
			return fmt.Errorf("failed to create media streams: %w", err)
		}

		return nil
	})
}

func (s *Service) SourceByID(ctx context.Context, id uuid.UUID) (*MediaSource, error) {
	source, err := s.store.ItemSource.GetModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query media source %s: %w", id, err)
	}

	return source, nil
}

func (s *Service) SourcesNeedingProbe(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := s.store.ItemSource.Query().
		Where(
			func(selector *sql.Selector) {
				probed, modified := selector.C(sourcemodel.FieldProbedAt), selector.C(sourcemodel.FieldDateModified)
				selector.Where(sql.Or(sql.IsNull(probed), sql.ColumnsLT(probed, modified)))
			},
		).
		Order(sourcemodel.ByPath()).
		IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the sources needing a probe: %w", err)
	}

	return ids, nil
}

func (s *Service) MediaSources(ctx context.Context, itemID uuid.UUID) ([]*MediaSource, error) {
	sources, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemID(itemID)).
		Order(sourcemodel.ByCreatedAt(), sourcemodel.ByPath()).
		WithStreams(func(query *store.MediaStreamQuery) {
			query.Order(streammodel.ByIndex())
		}).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query media sources: %w", err)
	}

	return sources, nil
}

type Held struct {
	Path         string
	RunTimeTicks *int64
	HasSubtitles bool
}

func (s *Service) FilesByItem(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]Held, error) {
	held := map[uuid.UUID]Held{}
	if len(itemIDs) == 0 {
		return held, nil
	}

	sources, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemIDIn(itemIDs...)).
		Order(sourcemodel.ByCreatedAt(), sourcemodel.ByPath()).
		WithStreams(func(query *store.MediaStreamQuery) {
			query.Where(streammodel.KindEQ(streammodel.KindSubtitle))
		}).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query media source paths: %w", err)
	}

	for _, source := range sources {
		entry, seen := held[source.ItemID]
		if !seen {
			entry = Held{Path: source.Path, RunTimeTicks: source.RunTimeTicks}
		}

		entry.HasSubtitles = entry.HasSubtitles || len(source.Streams) > 0
		held[source.ItemID] = entry
	}

	return held, nil
}

func (s *Service) SourcePaths(ctx context.Context, id uuid.UUID) ([]string, error) {
	ids, err := s.subtree(ctx, id)
	if err != nil {
		return nil, err
	}

	paths, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemIDIn(ids...)).
		Order(sourcemodel.ByPath()).
		Select(sourcemodel.FieldPath).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the paths under %s: %w", id, err)
	}

	return paths, nil
}

func (s *Service) DeleteSourcesNotInPaths(
	ctx context.Context,
	sourceID uuid.UUID,
	paths []string,
) ([]uuid.UUID, error) {
	where := []predicate.ItemSource{sourcemodel.SourceID(sourceID)}
	if len(paths) > 0 {
		where = append(where, sourcemodel.PathNotIn(paths...))
	}

	dropped, err := s.store.ItemSource.Query().
		Where(where...).
		Select(sourcemodel.FieldItemID).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select missing media sources: %w", err)
	}

	if _, err := s.store.ItemSource.Delete().Where(where...).Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to delete missing media sources: %w", err)
	}

	return parsed(dropped), nil
}

func NeedsProbe(source *MediaSource) bool {
	if source == nil || source.ProbedAt == nil {
		return true
	}

	return source.DateModified != nil && source.ProbedAt.Before(*source.DateModified)
}

func IsAudio(item *Item) bool {
	switch item.Kind {
	case itemmodel.KindAudio, itemmodel.KindAudioBook:
		return true
	default:
		return false
	}
}
