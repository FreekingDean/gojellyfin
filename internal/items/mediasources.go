package items

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

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
)

// What ffprobe found. The scan owns everything else on the item.
type Probe struct {
	Container    string
	RunTimeTicks int64
	Size         int64
	Bitrate      int32
	Streams      []Stream
	Metadata     ContainerMetadata
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
}

func (s *Service) SaveProbe(ctx context.Context, item *Item, probe Probe) error {
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

		if _, err := tx.MediaSource.Delete().Where(sourcemodal.ItemID(item.ID)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to clear media sources: %w", err)
		}

		source, err := tx.MediaSource.Create().
			SetItemID(item.ID).
			SetName(filepath.Base(item.Path)).
			SetPath(item.Path).
			SetContainer(probe.Container).
			SetRunTimeTicks(probe.RunTimeTicks).
			SetSize(probe.Size).
			SetBitrate(probe.Bitrate).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create media source: %w", err)
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
				SetIsForced(stream.IsForced))
		}
		if err := tx.MediaStream.CreateBulk(builders...).Exec(ctx); err != nil {
			return fmt.Errorf("failed to create media streams: %w", err)
		}

		return nil
	})
}

// Nil until the probe has run, which the caller has to stand in for.
func (s *Service) MediaSource(ctx context.Context, itemID uuid.UUID) (*MediaSource, error) {
	source, err := s.store.MediaSource.Query().
		Where(sourcemodal.ItemID(itemID)).
		WithStreams(func(query *store.MediaStreamQuery) {
			query.Order(streammodal.ByIndex())
		}).
		First(ctx)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query media source: %w", err)
	}

	return source, nil
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

// The probe is skipped unless the file changed since it last ran.
func NeedsProbe(item *Item) bool {
	return item.ProbedAt.IsZero() || item.ProbedAt.Before(item.DateModified)
}

func IsAudio(item *Item) bool {
	switch item.Kind {
	case itemmodal.KindAudio, itemmodal.KindAudioBook:
		return true
	default:
		return false
	}
}
