package segments

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	segmentmodel "github.com/FreekingDean/gojellyfin/internal/store/mediasegment"
)

const (
	analysedShare   = 4
	analysedLongest = 10 * time.Minute
	comparisons     = 4
)

type (
	Segment = store.MediaSegmentModel
	Kind    = segmentmodel.Kind
)

type Service struct {
	store  *store.Client
	ffmpeg *ffmpeg.FFMpeg
}

func New(client *store.Client, decoder *ffmpeg.FFMpeg) *Service {
	return &Service{store: client, ffmpeg: decoder}
}

func (s *Service) ForItem(ctx context.Context, id uuid.UUID, kinds []Kind) (uuid.UUID, []*Segment, error) {
	file, err := s.store.ItemSource.Query().Where(sourcemodel.ID(id)).WithSegments().OnlyModel(ctx)
	if err == nil {
		return file.ItemID, chosen(file.Segments, kinds), nil
	}
	if !store.IsNotFound(err) {
		return uuid.Nil, nil, fmt.Errorf("failed to query the segments of %s: %w", id, err)
	}

	files, err := s.store.ItemSource.Query().
		Where(sourcemodel.ItemID(id)).
		Order(sourcemodel.ByCreatedAt(), sourcemodel.ByPath()).
		WithSegments().
		AllModels(ctx)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("failed to query the segments of %s: %w", id, err)
	}

	for _, file := range files {
		if len(file.Segments) > 0 {
			return id, chosen(file.Segments, kinds), nil
		}
	}

	return id, []*Segment{}, nil
}

func chosen(held []*Segment, kinds []Kind) []*Segment {
	byKind := map[Kind]*Segment{}
	for _, segment := range held {
		if len(kinds) > 0 && !slices.Contains(kinds, segment.Kind) {
			continue
		}
		if current, ok := byKind[segment.Kind]; !ok || current.Origin != segmentmodel.OriginChapter {
			byKind[segment.Kind] = segment
		}
	}

	picked := make([]*Segment, 0, len(byKind))
	for _, segment := range byKind {
		picked = append(picked, segment)
	}
	slices.SortFunc(picked, func(a, b *Segment) int { return int(a.StartTicks - b.StartTicks) })

	return picked
}

func (s *Service) BatchJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.DetectIntros,
		Category:    "Library",
		Description: "Finds the intro every season's episodes share by listening to them.",
		Startable:   true,
		Queue:       jobs.SegmentsQueue,
		Run:         s.runBatch,
		Outstanding: s.outstanding,
	}
}

func (s *Service) outstanding(ctx context.Context) (int, error) {
	seasons, err := s.SeasonsNeedingDetection(ctx)

	return len(seasons), err
}

func (s *Service) SeasonJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.DetectSeasonIntros,
		Category:    "Library",
		Description: "Finds the intro one season's episodes share.",
		Queue:       jobs.SegmentsQueue,
		Run:         s.runSeason,
	}
}

func (s *Service) runBatch(ctx context.Context) error {
	seasons, err := s.SeasonsNeedingDetection(ctx)
	if err != nil {
		return err
	}

	for _, season := range seasons {
		if err := jobs.Enqueue(ctx, jobs.DetectSeasonIntros, jobs.With(jobs.ParamSeason, season)); err != nil {
			log.Printf("failed to enqueue %s: %v", jobs.DetectSeasonIntros, err)
		}
	}

	return nil
}

func (s *Service) runSeason(ctx context.Context) error {
	season, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamSeason)
	if err != nil {
		return err
	}

	return s.DetectSeason(ctx, season)
}

func (s *Service) SeasonsNeedingDetection(ctx context.Context) ([]uuid.UUID, error) {
	var rows []struct {
		ParentID uuid.UUID `json:"parent_id"`
	}

	err := s.store.Item.Query().
		Where(
			itemmodel.KindEQ(itemmodel.KindEpisode),
			itemmodel.DeletedAtIsNil(),
			itemmodel.ParentIDNotNil(),
			itemmodel.HasItemSourcesWith(sourcemodel.SegmentsDetectedAtIsNil()),
		).
		Unique(true).
		Select(itemmodel.FieldParentID).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to query the seasons needing intro detection: %w", err)
	}

	seasons := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		seasons = append(seasons, row.ParentID)
	}

	return seasons, nil
}

func (s *Service) DetectSeason(ctx context.Context, season uuid.UUID) error {
	stamped := map[uuid.UUID]bool{}
	for {
		files, err := s.seasonFiles(ctx, season)
		if err != nil {
			return err
		}

		var pending []uuid.UUID
		for _, file := range files {
			if file.SegmentsDetectedAt == nil && !stamped[file.ID] {
				pending = append(pending, file.ID)
			}
		}
		if len(pending) == 0 {
			return nil
		}

		if err := s.detect(ctx, files); err != nil {
			return err
		}

		err = s.store.ItemSource.Update().
			Where(sourcemodel.IDIn(pending...)).
			Modify(store.StampNow(sourcemodel.FieldSegmentsDetectedAt)).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to stamp the detected files: %w", err)
		}
		for _, id := range pending {
			stamped[id] = true
		}
	}
}

func (s *Service) seasonFiles(ctx context.Context, season uuid.UUID) ([]*items.MediaSource, error) {
	files, err := s.store.ItemSource.Query().
		Where(sourcemodel.HasItemWith(
			itemmodel.ParentID(season),
			itemmodel.KindEQ(itemmodel.KindEpisode),
			itemmodel.DeletedAtIsNil(),
		)).
		Order(sourcemodel.ByPath()).
		WithItem().
		WithSegments().
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the files of season %s: %w", season, err)
	}

	slices.SortStableFunc(files, func(a, b *items.MediaSource) int {
		return int(episodeNumber(a) - episodeNumber(b))
	})

	return files, nil
}

func episodeNumber(file *items.MediaSource) int32 {
	if file.Item == nil || file.Item.IndexNumber == nil {
		return 0
	}

	return *file.Item.IndexNumber
}

type listener struct {
	service *Service
	prints  map[uuid.UUID][]uint32
}

func (s *Service) detect(ctx context.Context, files []*items.MediaSource) error {
	heard := &listener{service: s, prints: map[uuid.UUID][]uint32{}}
	found := map[uuid.UUID]span{}

	for position, file := range files {
		if hasIntro(file) || found[file.ID] != (span{}) {
			continue
		}

		for _, other := range neighbours(files, position) {
			if err := ctx.Err(); err != nil {
				return err
			}
			jobs.Heartbeat(ctx, file.Path)

			mine, theirs, ok := heard.compare(ctx, file, other)
			if !ok {
				continue
			}

			found[file.ID] = mine
			if !hasIntro(other) && found[other.ID] == (span{}) {
				found[other.ID] = theirs
			}

			break
		}
	}

	for id, intro := range found {
		if err := s.saveIntro(ctx, id, intro); err != nil {
			return err
		}
	}

	return nil
}

func neighbours(files []*items.MediaSource, position int) []*items.MediaSource {
	var near []*items.MediaSource
	for distance := 1; distance < len(files) && len(near) < comparisons; distance++ {
		for _, at := range []int{position + distance, position - distance} {
			if at >= 0 && at < len(files) && files[at].ItemID != files[position].ItemID && len(near) < comparisons {
				near = append(near, files[at])
			}
		}
	}

	return near
}

func hasIntro(file *items.MediaSource) bool {
	return slices.ContainsFunc(file.Segments, func(segment *Segment) bool {
		return segment.Kind == segmentmodel.KindIntro
	})
}

func (l *listener) compare(ctx context.Context, file, other *items.MediaSource) (span, span, bool) {
	mine, theirs := l.print(ctx, file), l.print(ctx, other)
	if len(mine) == 0 || len(theirs) == 0 {
		return span{}, span{}, false
	}

	return shared(mine, theirs)
}

func (l *listener) print(ctx context.Context, file *items.MediaSource) []uint32 {
	if held, ok := l.prints[file.ID]; ok {
		return held
	}

	samples, err := l.service.ffmpeg.DecodeAudio(ctx, file.Path, analysed(file))
	if err != nil && !errors.Is(err, ffmpeg.ErrNotAvailable) {
		log.Printf("intro detection could not hear %s: %v", file.Path, err)
	}

	l.prints[file.ID] = fingerprint(samples)

	return l.prints[file.ID]
}

func analysed(file *items.MediaSource) time.Duration {
	if file.RunTimeTicks == nil || *file.RunTimeTicks <= 0 {
		return analysedLongest
	}

	return min(analysedLongest, time.Duration(*file.RunTimeTicks*100)/analysedShare)
}

func (s *Service) saveIntro(ctx context.Context, fileID uuid.UUID, intro span) error {
	err := s.store.MediaSegment.Create().
		SetItemSourceID(fileID).
		SetKind(segmentmodel.KindIntro).
		SetOrigin(segmentmodel.OriginFingerprint).
		SetStartTicks(ticks(intro.start)).
		SetEndTicks(ticks(intro.end)).
		OnConflictColumns(segmentmodel.FieldItemSourceID, segmentmodel.FieldOrigin, segmentmodel.FieldKind).
		UpdateStartTicks().
		UpdateEndTicks().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save the intro of %s: %w", fileID, err)
	}

	return nil
}

func ticks(at time.Duration) int64 {
	return int64(at / 100)
}
