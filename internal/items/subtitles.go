package items

import (
	"context"
	"fmt"
	"path/filepath"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

type ExternalSubtitle struct {
	Path              string
	Language          string
	Title             string
	Codec             string
	IsDefault         bool
	IsForced          bool
	IsHearingImpaired bool
}

// External streams are indexed off the end of the container's own, so the
// source row is locked for the whole rewrite; without it two writers allocate
// the same index.
func (s *Service) ReplaceExternalSubtitles(ctx context.Context, item *Item, subtitles []ExternalSubtitle) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		source, err := tx.MediaSource.Query().
			Where(sourcemodal.ItemID(item.ID)).
			ForUpdate().
			First(ctx)
		if store.IsNotFound(err) && len(subtitles) == 0 {
			return tx.Item.UpdateOneID(item.ID).SetHasSubtitles(false).Exec(ctx)
		}
		if store.IsNotFound(err) {
			source, err = tx.MediaSource.Create().
				SetItemID(item.ID).
				SetName(filepath.Base(item.Path)).
				SetPath(item.Path).
				Save(ctx)
		}
		if err != nil {
			return fmt.Errorf("failed to resolve the media source: %w", err)
		}

		_, err = tx.MediaStream.Delete().
			Where(
				streammodal.SourceID(source.ID),
				streammodal.KindEQ(streammodal.KindSubtitle),
				streammodal.IsExternal(true),
			).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to clear external subtitles: %w", err)
		}

		index, err := nextStreamIndex(ctx, tx, source.ID)
		if err != nil {
			return err
		}

		builders := make([]*store.MediaStreamCreate, 0, len(subtitles))
		for _, subtitle := range subtitles {
			builders = append(builders, tx.MediaStream.Create().
				SetSourceID(source.ID).
				SetIndex(index).
				SetKind(streammodal.KindSubtitle).
				SetPath(subtitle.Path).
				SetCodec(subtitle.Codec).
				SetLanguage(subtitle.Language).
				SetTitle(subtitle.Title).
				SetIsExternal(true).
				SetIsDefault(subtitle.IsDefault).
				SetIsForced(subtitle.IsForced).
				SetIsHearingImpaired(subtitle.IsHearingImpaired))
			index++
		}
		if err := tx.MediaStream.CreateBulk(builders...).Exec(ctx); err != nil {
			return fmt.Errorf("failed to create external subtitles: %w", err)
		}

		count, err := tx.MediaStream.Query().
			Where(streammodal.SourceID(source.ID), streammodal.KindEQ(streammodal.KindSubtitle)).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to count subtitles: %w", err)
		}

		return tx.Item.UpdateOneID(item.ID).SetHasSubtitles(count > 0).Exec(ctx)
	})
}

func nextStreamIndex(ctx context.Context, tx *store.Tx, sourceID uuid.UUID) (int32, error) {
	highest, err := tx.MediaStream.Query().
		Where(streammodal.SourceID(sourceID)).
		Order(streammodal.ByIndex(sql.OrderDesc())).
		First(ctx)
	if store.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query the highest stream index: %w", err)
	}

	return highest.Index + 1, nil
}

func (s *Service) SubtitleStream(ctx context.Context, itemID uuid.UUID, index int32) (*MediaStream, error) {
	stream, err := s.store.MediaStream.Query().
		Where(
			streammodal.HasSourceWith(sourcemodal.ItemID(itemID)),
			streammodal.KindEQ(streammodal.KindSubtitle),
			streammodal.Index(index),
		).
		First(ctx)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query subtitle stream: %w", err)
	}

	return stream, nil
}
