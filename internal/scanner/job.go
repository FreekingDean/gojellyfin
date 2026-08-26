package scanner

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

const (
	RefreshLibraryJobID = "RefreshLibrary"
	probeChunkSize      = 100
)

type LibraryScan struct {
	scanner *Scanner
}

func NewLibraryScan(scanner *Scanner) *LibraryScan {
	return &LibraryScan{scanner: scanner}
}

func (l *LibraryScan) Name() string     { return RefreshLibraryJobID }
func (l *LibraryScan) Category() string { return "Library" }
func (l *LibraryScan) Description() string {
	return "Scans the media libraries for new and changed files."
}

func (l *LibraryScan) Steps() []any {
	return []any{
		l.scanner.ListLibraries,
		l.scanner.ScanLibrary,
		l.scanner.UnprobedSources,
		l.scanner.ProbeSource,
	}
}

func (l *LibraryScan) Children() []any {
	return []any{l.ProbeChunk}
}

func (l *LibraryScan) Run(ctx jobs.Context, _ jobs.Options) error {
	var libraries []uuid.UUID
	if err := jobs.Step(ctx, l.scanner.ListLibraries).Get(&libraries); err != nil {
		return err
	}

	scans := make([]jobs.Future, 0, len(libraries))
	for _, id := range libraries {
		scans = append(scans, jobs.Step(ctx, l.scanner.ScanLibrary, id))
	}

	walked := make([]uuid.UUID, 0, len(libraries))
	for index, scan := range scans {
		if err := scan.Get(nil); err != nil {
			jobs.Logf(ctx, "library scan failed", "library", libraries[index], "error", err)

			continue
		}
		walked = append(walked, libraries[index])
	}

	return l.probe(ctx, walked)
}

func (l *LibraryScan) probe(ctx jobs.Context, libraries []uuid.UUID) error {
	selections := make([]jobs.Future, 0, len(libraries))
	for _, id := range libraries {
		selections = append(selections, jobs.Step(ctx, l.scanner.UnprobedSources, id))
	}

	chunks := make([]jobs.Future, 0)
	for index, selection := range selections {
		var sources []uuid.UUID
		if err := selection.Get(&sources); err != nil {
			jobs.Logf(ctx, "probe selection failed", "library", libraries[index], "error", err)

			continue
		}

		number := 0
		for chunk := range slices.Chunk(sources, probeChunkSize) {
			name := fmt.Sprintf("probe-%s-%d", libraries[index], number)
			chunks = append(chunks, jobs.Child(ctx, l.ProbeChunk, name, chunk))
			number++
		}
	}

	for _, chunk := range chunks {
		if err := chunk.Get(nil); err != nil {
			jobs.Logf(ctx, "probe chunk failed", "error", err)
		}
	}

	return nil
}

func (l *LibraryScan) ProbeChunk(ctx jobs.Context, sources []uuid.UUID) error {
	for _, id := range sources {
		if err := jobs.Step(ctx, l.scanner.ProbeSource, id).Get(nil); err != nil {
			jobs.Logf(ctx, "probe failed", "source", id, "error", err)
		}
	}

	return nil
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

func (s *Scanner) ScanLibrary(ctx context.Context, id uuid.UUID) error {
	library, err := s.libraries.Library(ctx, id)
	if err != nil {
		return err
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
