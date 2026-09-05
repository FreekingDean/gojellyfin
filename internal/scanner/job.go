package scanner

import (
	"context"
	"log"
	"runtime"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

const RefreshLibraryJobID = "RefreshLibrary"

type LibraryScan struct {
	scanner *Scanner
}

func NewLibraryScan(scanner *Scanner) *LibraryScan {
	return &LibraryScan{scanner: scanner}
}

func (l *LibraryScan) Name() string     { return RefreshLibraryJobID }
func (l *LibraryScan) Category() string { return "Library" }
func (l *LibraryScan) Description() string {
	return "Reads the libraries from their Sonarr and Radarr sources."
}

func (l *LibraryScan) Run(ctx context.Context) error {
	libraries, err := l.scanner.ListLibraries(ctx)
	if err != nil {
		return err
	}

	disturbed := make([]uuid.UUID, 0)
	for _, id := range libraries {
		dropped, err := l.scanner.ScanLibrary(ctx, id)
		if err != nil {
			log.Printf("library scan failed %s: %v", id, err)

			continue
		}
		disturbed = append(disturbed, dropped...)
	}

	configured, err := l.scanner.sources.List(ctx)
	if err != nil {
		return err
	}

	for _, entry := range configured {
		if err := l.probe(ctx, entry.Source.ID); err != nil {
			log.Printf("probe failed %s: %v", entry.Source.Name, err)
		}
	}

	return l.scanner.items.SweepUnreachable(ctx, disturbed)
}

func (l *LibraryScan) probe(ctx context.Context, source uuid.UUID) error {
	files, err := l.scanner.UnprobedSources(ctx, source)
	if err != nil {
		return err
	}

	group, probing := errgroup.WithContext(ctx)
	group.SetLimit(runtime.GOMAXPROCS(0))

	for _, file := range files {
		group.Go(func() error {
			if err := l.scanner.ProbeSource(probing, file); err != nil && !store.IsNotFound(err) {
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

	item, err := s.items.ItemByID(ctx, source.ItemID)
	if err != nil {
		return err
	}

	return s.items.SaveProbe(ctx, item, source, *probe)
}
