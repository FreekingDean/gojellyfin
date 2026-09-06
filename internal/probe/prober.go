package probe

import (
	"context"
	"log"
	"runtime"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

type Prober struct {
	items  *items.Service
	ffmpeg *ffmpeg.FFMpeg
}

func New(records *items.Service, probe *ffmpeg.FFMpeg) *Prober {
	return &Prober{items: records, ffmpeg: probe}
}

func (s *Prober) FileJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.ProbeFile,
		Category:    "Library",
		Description: "Reads the streams of one file.",
		Run:         s.runOne,
	}
}

func (s *Prober) runOne(ctx context.Context) error {
	id, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamSource)
	if err != nil {
		return err
	}

	if err := s.ProbeSource(ctx, id); err != nil && !store.IsNotFound(err) {
		return err
	}

	return nil
}

func (s *Prober) Job() jobs.Job {
	return jobs.Job{
		Name:        jobs.ProbeFiles,
		Category:    "Library",
		Description: "Reads the streams of every file it has not seen before.",
		Run:         s.run,
	}
}

func (s *Prober) run(ctx context.Context) error {
	files, err := s.items.SourcesNeedingProbe(ctx)
	if err != nil {
		return err
	}

	group, probing := errgroup.WithContext(ctx)
	group.SetLimit(runtime.GOMAXPROCS(0))

	for _, file := range files {
		group.Go(func() error {
			if err := s.ProbeSource(probing, file); err != nil && !store.IsNotFound(err) {
				log.Printf("probe failed %s: %v", file, err)
			}

			return probing.Err()
		})
	}

	return group.Wait()
}

func (s *Prober) ProbeSource(ctx context.Context, id uuid.UUID) error {
	source, err := s.items.SourceByID(ctx, id)
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	jobs.Heartbeat(ctx, source.Path)
	if err := ctx.Err(); err != nil {
		return err
	}

	probed, err := s.probeFile(ctx, source)
	if err != nil || probed == nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	item, err := s.items.ItemByID(ctx, items.Everyone, source.ItemID)
	if err != nil {
		return err
	}

	return s.items.SaveProbe(ctx, item, source, *probed)
}
