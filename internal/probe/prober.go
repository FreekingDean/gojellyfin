package probe

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

type Reports interface {
	File(ctx context.Context, filing reports.Filing) error
}

type Prober struct {
	items   *items.Service
	ffmpeg  *ffmpeg.FFMpeg
	reports Reports
}

func New(records *items.Service, probe *ffmpeg.FFMpeg, filed *reports.Service) *Prober {
	return &Prober{items: records, ffmpeg: probe, reports: filed}
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
		Startable:   true,
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

func unreadable(err error, path string) bool {
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		return false
	}

	_, missing := os.Stat(path)

	return missing == nil
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
	if unreadable(err, source.Path) {
		return errors.Join(err, s.reports.File(ctx, reports.Filing{
			ItemID: source.ItemID,
			FileID: &source.ID,
			Kind:   reports.KindUnplayable,
			Origin: reports.OriginProbe,
			Note:   err.Error(),
		}))
	}
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
