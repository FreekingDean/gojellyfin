package scanner

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/google/uuid"
)

const (
	RefreshLibraryJobID = "RefreshLibrary"
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
	return []any{l.scanner.ListLibraries, l.scanner.ScanLibrary}
}

// A library is a handful of rows, so the fan out is flat. Per item work is what
// would need chunking, and none exists yet.
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
	for index, scan := range scans {
		if err := scan.Get(nil); err != nil {
			jobs.Logf(ctx, "library scan failed", "library", libraries[index], "error", err)
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

// Heartbeats so that a scan which is merely slow is told apart from a worker
// that died, which is the same distinction TRANSCODER_STALL_TIMEOUT draws.
func (s *Scanner) ScanLibrary(ctx context.Context, id uuid.UUID) error {
	library, err := s.libraries.Library(ctx, id)
	if err != nil {
		return err
	}

	jobs.Heartbeat(ctx, library.Name)

	return s.scanLibrary(ctx, library)
}
