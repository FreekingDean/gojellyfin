package items

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	keyframemodel "github.com/FreekingDean/gojellyfin/internal/store/keyframeindex"
	segmentmodel "github.com/FreekingDean/gojellyfin/internal/store/mediasegment"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
)

type (
	MediaSource = store.ItemSourceModel
	MediaStream = store.MediaStreamModel
	StreamKind  = streammodel.Kind

	VideoRangeType = streammodel.VideoRangeType

	MediaSegment  = store.MediaSegmentModel
	KeyframeIndex = store.KeyframeIndexModel
	SegmentKind   = segmentmodel.Kind
	SegmentOrigin = segmentmodel.Origin
)

func (s *Service) SaveSource(ctx context.Context, scanned MediaSource) (*MediaSource, error) {
	id, err := s.store.ItemSource.Create().
		SetSourceID(scanned.SourceID).
		SetItemID(scanned.ItemID).
		SetPath(scanned.Path).
		SetName(scanned.Name).
		SetNillableDateModified(scanned.DateModified).
		SetNillableExternalID(scanned.ExternalID).
		SetNillableExternalFileID(scanned.ExternalFileID).
		OnConflictColumns(sourcemodel.FieldPath).
		UpdateItemID().
		UpdateSourceID().
		UpdateName().
		UpdateDateModified().
		UpdateExternalID().
		UpdateExternalFileID().
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

func (s *Service) fileOwner(ctx context.Context, path string) (*uuid.UUID, error) {
	owners, err := s.store.ItemSource.Query().Where(sourcemodel.Path(path)).Select(sourcemodel.FieldItemID).Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query who holds %s: %w", path, err)
	}
	if len(owners) == 0 {
		return nil, nil
	}

	owner, err := uuid.Parse(owners[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read who holds %s: %w", path, err)
	}

	return &owner, nil
}

func (s *Service) SaveProbe(ctx context.Context, item *Item, source *MediaSource, probe MediaSource) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		err := tx.ItemSource.UpdateOneID(source.ID).
			SetContainer(probe.Container).
			SetNillableRunTimeTicks(probe.RunTimeTicks).
			SetNillableSize(probe.Size).
			SetNillableBitrate(probe.Bitrate).
			SetLyrics(probe.Lyrics).
			Modify(store.StampNow(sourcemodel.FieldProbedAt)).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to save the probed media source: %w", err)
		}

		if err := saveChapterSegments(ctx, tx, source.ID, probe.Segments); err != nil {
			return err
		}

		if err := saveKeyframes(ctx, tx, source.ID, probe.Keyframes); err != nil {
			return err
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

func saveChapterSegments(ctx context.Context, tx *store.Tx, sourceID uuid.UUID, found []*MediaSegment) error {
	_, err := tx.MediaSegment.Delete().
		Where(segmentmodel.ItemSourceID(sourceID), segmentmodel.OriginEQ(segmentmodel.OriginChapter)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear chapter segments: %w", err)
	}

	if len(found) == 0 {
		return nil
	}

	builders := make([]*store.MediaSegmentCreate, 0, len(found))
	for _, segment := range found {
		builders = append(builders, tx.MediaSegment.Create().
			SetItemSourceID(sourceID).
			SetKind(segment.Kind).
			SetOrigin(segmentmodel.OriginChapter).
			SetStartTicks(segment.StartTicks).
			SetEndTicks(segment.EndTicks))
	}
	if err := tx.MediaSegment.CreateBulk(builders...).Exec(ctx); err != nil {
		return fmt.Errorf("failed to create chapter segments: %w", err)
	}

	return nil
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
		WithSegments().
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query media sources: %w", err)
	}

	return sources, nil
}

func (s *Service) PlayedSources(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]*MediaSource, error) {
	return s.richest(ctx, itemIDs, func(*MediaSource) bool { return true })
}

func (s *Service) EncodableSources(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]*MediaSource, error) {
	return s.richest(ctx, itemIDs, s.encodable)
}

func (s *Service) richest(ctx context.Context, itemIDs []uuid.UUID, takes func(*MediaSource) bool) (map[uuid.UUID]*MediaSource, error) {
	played := map[uuid.UUID]*MediaSource{}
	if len(itemIDs) == 0 {
		return played, nil
	}

	sources, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemIDIn(itemIDs...)).
		Order(sourcemodel.ByCreatedAt(), sourcemodel.ByPath()).
		WithStreams(func(query *store.MediaStreamQuery) {
			query.Order(streammodel.ByIndex())
		}).
		WithSegments().
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the media sources of items: %w", err)
	}

	for _, source := range sources {
		if !takes(source) {
			continue
		}
		if listed, seen := played[source.ItemID]; !seen || richer(source, listed) {
			played[source.ItemID] = source
		}
	}

	return played, nil
}

type Held struct {
	RunTimeTicks *int64
	HasSubtitles bool
	HasLyrics    bool
}

func (s *Service) FilesByItem(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]Held, error) {
	held := map[uuid.UUID]Held{}
	if len(itemIDs) == 0 {
		return held, nil
	}

	sources, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemIDIn(itemIDs...)).
		Order(sourcemodel.ByCreatedAt(), sourcemodel.ByPath()).
		Select(sourcemodel.FieldID, sourcemodel.FieldItemID, sourcemodel.FieldRunTimeTicks).
		WithStreams(func(query *store.MediaStreamQuery) {
			query.Where(streammodel.KindEQ(streammodel.KindSubtitle))
		}).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query media source paths: %w", err)
	}

	sung, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemIDIn(itemIDs...), sourcemodel.LyricsNotNil(), sourcemodel.LyricsNEQ("")).
		Unique(true).
		Select(sourcemodel.FieldItemID).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query which items have lyrics: %w", err)
	}

	for _, source := range sources {
		entry, seen := held[source.ItemID]
		if !seen {
			entry = Held{RunTimeTicks: source.RunTimeTicks}
		}

		entry.HasSubtitles = entry.HasSubtitles || len(source.Streams) > 0
		held[source.ItemID] = entry
	}

	for _, raw := range sung {
		if id, err := uuid.Parse(raw); err == nil {
			entry := held[id]
			entry.HasLyrics = true
			held[id] = entry
		}
	}

	return held, nil
}

func (s *Service) Lyrics(ctx context.Context, viewer Viewer, itemID uuid.UUID) (string, error) {
	_, err := s.ItemByID(ctx, viewer, itemID)
	if store.IsNotFound(err) {
		return "", fmt.Errorf("%w: no item %s", ErrNotFound, itemID)
	}
	if err != nil {
		return "", err
	}

	found, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemID(itemID), sourcemodel.LyricsNotNil(), sourcemodel.LyricsNEQ("")).
		Order(sourcemodel.ByCreatedAt(), sourcemodel.ByPath()).
		Select(sourcemodel.FieldLyrics).
		Strings(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to query the lyrics: %w", err)
	}
	if len(found) == 0 {
		return "", fmt.Errorf("%w: no file of %s carries lyrics", ErrNotFound, itemID)
	}

	return found[0], nil
}

func (s *Service) SaveLyrics(ctx context.Context, viewer Viewer, itemID uuid.UUID, text string) error {
	item, err := s.ItemByID(ctx, viewer, itemID)
	if store.IsNotFound(err) || (err == nil && !IsAudio(item)) {
		return fmt.Errorf("%w: no track %s", ErrNotFound, itemID)
	}
	if err != nil {
		return err
	}

	first, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemID(itemID)).
		Order(sourcemodel.ByCreatedAt(), sourcemodel.ByPath()).
		FirstID(ctx)
	if store.IsNotFound(err) {
		return fmt.Errorf("%w: no file of %s to hold lyrics", ErrNotFound, itemID)
	}
	if err != nil {
		return fmt.Errorf("failed to find the file to hold the lyrics: %w", err)
	}

	if err := s.store.ItemSource.UpdateOneID(first).SetLyrics(text).Exec(ctx); err != nil {
		return fmt.Errorf("failed to save the lyrics: %w", err)
	}

	return nil
}

func (s *Service) ClearLyrics(ctx context.Context, viewer Viewer, itemID uuid.UUID) error {
	if _, err := s.Lyrics(ctx, viewer, itemID); err != nil {
		return err
	}

	if err := s.store.ItemSource.Update().
		Where(sourcemodel.ItemID(itemID)).
		ClearLyrics().
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear the lyrics: %w", err)
	}

	return nil
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
	return s.deleteSourcesNotIn(ctx, paths, sourcemodel.SourceID(sourceID))
}

func (s *Service) DeleteScopedSourcesNotInPaths(
	ctx context.Context,
	sourceID uuid.UUID,
	scope Scope,
	paths []string,
) ([]uuid.UUID, error) {
	return s.deleteSourcesNotIn(ctx, paths, sourcemodel.SourceID(sourceID), sourcemodel.HasItemWith(scope.items()))
}

func (s *Service) deleteSourcesNotIn(ctx context.Context, paths []string, where ...predicate.ItemSource) ([]uuid.UUID, error) {
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
	return item.Kind == itemmodel.KindAudio
}

func saveKeyframes(ctx context.Context, tx *store.Tx, sourceID uuid.UUID, index *KeyframeIndex) error {
	if _, err := tx.KeyframeIndex.Delete().Where(keyframemodel.ItemSourceID(sourceID)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear the keyframe index: %w", err)
	}
	if index == nil {
		return nil
	}

	if err := tx.KeyframeIndex.Create().SetItemSourceID(sourceID).SetTicks(index.Ticks).Exec(ctx); err != nil {
		return fmt.Errorf("failed to save the keyframe index: %w", err)
	}

	return nil
}

func (s *Service) Keyframes(ctx context.Context, sourceID uuid.UUID) ([]int64, error) {
	index, err := s.store.KeyframeIndex.Query().Where(keyframemodel.ItemSourceID(sourceID)).OnlyModel(ctx)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query the keyframe index: %w", err)
	}

	return index.Ticks, nil
}
