package items

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

type (
	MediaSource = store.MediaSource
	MediaStream = store.MediaStream
	StreamKind  = streammodal.Kind

	VideoRangeType = streammodal.VideoRangeType
)

// Where one file of a title lives. The probe owns the rest of the row.
type ScannedSource struct {
	LibraryID    uuid.UUID
	ItemID       uuid.UUID
	Path         string
	Name         string
	DateModified time.Time
}

// What ffprobe found. The scan owns everything else on the item.
type Probe struct {
	Container    string
	RunTimeTicks int64
	Size         int64
	Bitrate      int32
	Streams      []Stream
	Metadata     ContainerMetadata
}

// A picture nobody could read the range of is left unset rather than called
// SDR: an unread property has to stay unknown, because a client's conditions
// are answered by passing what nobody probed rather than by guessing at it.
func nillableRangeType(rangeType VideoRangeType) *VideoRangeType {
	if rangeType == "" {
		return nil
	}

	return &rangeType
}

type Stream struct {
	Index       int32
	Kind        StreamKind
	Codec       string
	Profile     string
	Language    string
	Title       string
	Width       int32
	Height      int32
	Channels    int32
	SampleRate  int32
	Bitrate     int32
	PixelFormat string
	Level       float64
	IsDefault   bool
	IsForced    bool

	RangeType    VideoRangeType
	IsInterlaced bool
	IsAnamorphic bool
}

// The probe columns are left out of the update: a file that has not changed
// keeps the runtime and streams the last probe wrote, and that runtime is what
// decided which item the file belongs to.
func (s *Service) SaveSource(ctx context.Context, scanned ScannedSource) (*MediaSource, error) {
	id, err := s.store.MediaSource.Create().
		SetLibraryID(scanned.LibraryID).
		SetItemID(scanned.ItemID).
		SetPath(scanned.Path).
		SetName(scanned.Name).
		SetDateModified(scanned.DateModified).
		OnConflictColumns(sourcemodal.FieldLibraryID, sourcemodal.FieldPath).
		UpdateItemID().
		UpdateName().
		UpdateDateModified().
		UpdateUpdatedAt().
		ID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to save media source: %w", err)
	}

	source, err := s.store.MediaSource.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query the saved media source: %w", err)
	}

	return source, nil
}

func (s *Service) SaveProbe(ctx context.Context, item *Item, source *MediaSource, probe Probe) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		genres, err := genreIDs(ctx, tx, probe.Metadata.Genres)
		if err != nil {
			return err
		}
		studios, err := studioIDs(ctx, tx, probe.Metadata.Studios)
		if err != nil {
			return err
		}

		err = tx.Item.UpdateOneID(item.ID).
			SetContainer(probe.Container).
			SetRunTimeTicks(probe.RunTimeTicks).
			SetProbedAt(time.Now()).
			SetTags(probe.Metadata.Tags).
			ClearGenres().
			AddGenreIDs(genres...).
			ClearStudios().
			AddStudioIDs(studios...).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to save probed item: %w", err)
		}

		if err := saveCredits(ctx, tx, item.ID, probe.Metadata.People); err != nil {
			return err
		}

		err = tx.MediaSource.UpdateOneID(source.ID).
			SetContainer(probe.Container).
			SetRunTimeTicks(probe.RunTimeTicks).
			SetSize(probe.Size).
			SetBitrate(probe.Bitrate).
			SetProbedAt(time.Now()).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to save the probed media source: %w", err)
		}

		if _, err := tx.MediaStream.Delete().Where(streammodal.SourceID(source.ID)).Exec(ctx); err != nil {
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
				SetWidth(stream.Width).
				SetHeight(stream.Height).
				SetChannels(stream.Channels).
				SetSampleRate(stream.SampleRate).
				SetBitRate(stream.Bitrate).
				SetPixelFormat(stream.PixelFormat).
				SetLevel(stream.Level).
				SetIsDefault(stream.IsDefault).
				SetIsForced(stream.IsForced).
				SetNillableVideoRangeType(nillableRangeType(stream.RangeType)).
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
	source, err := s.store.MediaSource.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query media source %s: %w", id, err)
	}

	return source, nil
}

// The files whose probe is missing or older than the file itself. Work is
// selected from the rows rather than remembered from the walk that wrote them,
// so a run that died can be told what is outstanding rather than replaying it.
func (s *Service) SourcesNeedingProbe(ctx context.Context, libraryID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.store.MediaSource.Query().
		Where(
			sourcemodal.LibraryID(libraryID),
			func(selector *sql.Selector) {
				probed, modified := selector.C(sourcemodal.FieldProbedAt), selector.C(sourcemodal.FieldDateModified)
				selector.Where(sql.Or(sql.IsNull(probed), sql.ColumnsLT(probed, modified)))
			},
		).
		Order(sourcemodal.ByPath()).
		IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the sources needing a probe: %w", err)
	}

	return ids, nil
}

// Every file the item plays from, oldest first, so a caller that can only use
// one takes the first and the ordering does not shift under it.
func (s *Service) MediaSources(ctx context.Context, itemID uuid.UUID) ([]*MediaSource, error) {
	sources, err := s.store.MediaSource.Query().
		Where(sourcemodal.ItemID(itemID)).
		Order(sourcemodal.ByCreatedAt(), sourcemodal.ByPath()).
		WithStreams(func(query *store.MediaStreamQuery) {
			query.Order(streammodal.ByIndex())
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query media sources: %w", err)
	}

	return sources, nil
}

// The primary file of each item, for list responses that would otherwise ask
// per row.
func (s *Service) PathsByItem(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	paths := map[uuid.UUID]string{}
	if len(itemIDs) == 0 {
		return paths, nil
	}

	sources, err := s.store.MediaSource.Query().
		Where(sourcemodal.ItemIDIn(itemIDs...)).
		Order(sourcemodal.ByCreatedAt(), sourcemodal.ByPath()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query media source paths: %w", err)
	}

	for _, source := range sources {
		if _, ok := paths[source.ItemID]; !ok {
			paths[source.ItemID] = source.Path
		}
	}

	return paths, nil
}

// Every file under an item, including the ones its children play from, which is
// what deleting a series has to reach.
func (s *Service) SourcePaths(ctx context.Context, id uuid.UUID) ([]string, error) {
	ids, err := s.subtree(ctx, id)
	if err != nil {
		return nil, err
	}

	paths, err := s.store.MediaSource.Query().
		Where(sourcemodal.ItemIDIn(ids...)).
		Order(sourcemodal.ByPath()).
		Select(sourcemodal.FieldPath).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the paths under %s: %w", id, err)
	}

	return paths, nil
}

// A source carries no watch state, so a file that goes away takes its row with
// it; the item keeps the id everything else hangs off and is swept separately.
func (s *Service) DeleteSourcesNotInPaths(ctx context.Context, libraryID uuid.UUID, paths []string) error {
	missing := s.store.MediaSource.Delete().Where(sourcemodal.LibraryID(libraryID))
	if len(paths) > 0 {
		missing = missing.Where(sourcemodal.PathNotIn(paths...))
	}

	if _, err := missing.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete missing media sources: %w", err)
	}

	return nil
}

// Empty until the probe has run, which the caller has to treat as unknown.
func (s *Service) AudioCodec(ctx context.Context, itemID uuid.UUID) (string, error) {
	codecs, err := s.store.MediaStream.Query().
		Where(
			streammodal.KindEQ(streammodal.KindAudio),
			streammodal.HasSourceWith(sourcemodal.ItemID(itemID)),
		).
		Order(streammodal.ByIndex()).
		Limit(1).
		Select(streammodal.FieldCodec).
		Strings(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to query audio codec: %w", err)
	}
	if len(codecs) == 0 {
		return "", nil
	}

	return codecs[0], nil
}

// The same question SourcesNeedingProbe asks of the whole library, asked of one
// row: the selection and the probe are minutes apart and the file can have been
// read in between.
func NeedsProbe(source *MediaSource) bool {
	return source == nil || source.ProbedAt.IsZero() || source.ProbedAt.Before(source.DateModified)
}

func IsAudio(item *Item) bool {
	switch item.Kind {
	case itemmodal.KindAudio, itemmodal.KindAudioBook:
		return true
	default:
		return false
	}
}
