package scanner

import (
	"context"
	"log"
	"runtime"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

const RefreshLibraryJobID = "RefreshLibrary"

func (s *Scanner) Job() jobs.Job {
	return jobs.Job{
		Name:        RefreshLibraryJobID,
		Category:    "Library",
		Description: "Reads the libraries from their Sonarr and Radarr sources.",
		Run:         s.refresh,
	}
}

func (s *Scanner) refresh(ctx context.Context) error {
	libraries, err := s.ListLibraries(ctx)
	if err != nil {
		return err
	}

	disturbed := make([]uuid.UUID, 0)
	for _, id := range libraries {
		dropped, err := s.ScanLibrary(ctx, id)
		if err != nil {
			log.Printf("library scan failed %s: %v", id, err)

			continue
		}
		disturbed = append(disturbed, dropped...)
	}

	configured, err := s.sources.List(ctx)
	if err != nil {
		return err
	}

	for _, entry := range configured {
		if err := s.probe(ctx, entry.Source.ID); err != nil {
			log.Printf("probe failed %s: %v", entry.Source.Name, err)
		}
	}

	return s.items.SweepUnreachable(ctx, disturbed)
}

func (s *Scanner) probe(ctx context.Context, source uuid.UUID) error {
	files, err := s.UnprobedSources(ctx, source)
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

func (s *Scanner) ListLibraries(ctx context.Context) ([]uuid.UUID, error) {
	scanned, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(scanned))
	for _, library := range scanned {
		ids = append(ids, library.ID)
	}

	return ids, nil
}

func (s *Scanner) ScanLibrary(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	library, err := s.libraries.Library(ctx, id)
	if err != nil {
		return nil, err
	}

	jobs.Heartbeat(ctx, library.Name)

	return s.scanLibrary(ctx, library)
}

func (s *Scanner) UnprobedSources(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	return s.items.SourcesNeedingProbe(ctx, id)
}

func (s *Scanner) ProbeSource(ctx context.Context, id uuid.UUID) error {
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

	probe, err := s.probeFile(ctx, source)
	if err != nil {
		return err
	}
	if probe == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	item, err := s.items.ItemByID(ctx, items.Everyone, source.ItemID)
	if err != nil {
		return err
	}

	return s.items.SaveProbe(ctx, item, source, *probe)
}
