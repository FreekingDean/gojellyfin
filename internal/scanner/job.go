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

// A library is a handful of rows, so the walk fans out flat. What follows it is
// per file and there are thousands, so the probes fan out through children.
func (l *LibraryScan) Run(ctx jobs.Context) error {
	var libraries []uuid.UUID
	if err := jobs.Step(ctx, l.scanner.ListLibraries).Get(&libraries); err != nil {
		return err
	}

	scans := make([]jobs.Future, 0, len(libraries))
	for _, id := range libraries {
		scans = append(scans, jobs.Step(ctx, l.scanner.ScanLibrary, id))
	}

	// One unreadable library must not abandon the others, and the sweep it
	// skipped is safe to leave until the next run.
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

// The files to probe are selected from the rows rather than handed down by the
// walk, so a run that died leaves the next one able to work out what is still
// outstanding instead of replaying what the walk happened to touch.
func (l *LibraryScan) probe(ctx jobs.Context, libraries []uuid.UUID) error {
	selections := make([]jobs.Future, 0, len(libraries))
	for _, id := range libraries {
		selections = append(selections, jobs.Step(ctx, l.scanner.UnprobedSources, id))
	}

	names := make([]string, 0)
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
			names = append(names, name)
			chunks = append(chunks, jobs.Child(ctx, l.ProbeChunk, name, chunk))
			number++
		}
	}

	for index, chunk := range chunks {
		if err := chunk.Get(nil); err != nil {
			jobs.Logf(ctx, "probe chunk failed", "chunk", names[index], "error", err)
		}
	}

	return nil
}

// One file at a time, because a probe saturates about a core and the parallelism
// worth having is across chunks rather than inside one.
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

// A file that went away between the selection and the probe is done rather than
// failed: the sweep has already taken its row.
func (s *Scanner) ProbeSource(ctx context.Context, id uuid.UUID) error {
	source, err := s.items.SourceByID(ctx, id)
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	jobs.Heartbeat(ctx, source.Path)

	probe, err := s.probeFile(ctx, source)
	if err != nil {
		return err
	}
	if probe == nil {
		return nil
	}

	item, err := s.items.ItemByID(ctx, source.ItemID)
	if err != nil {
		return err
	}

	return s.items.SaveProbe(ctx, item, source, *probe)
}
