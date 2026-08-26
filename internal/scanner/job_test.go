package scanner

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

type run struct {
	scan      *LibraryScan
	env       *jobs.TestEnvironment
	libraries []uuid.UUID
	sources   map[uuid.UUID][]uuid.UUID

	mutex   sync.Mutex
	walked  []uuid.UUID
	probed  []uuid.UUID
	failing map[uuid.UUID]error
}

func newRun(t *testing.T, libraries ...uuid.UUID) *run {
	t.Helper()

	running := &run{
		scan:      NewLibraryScan(&Scanner{}),
		env:       jobs.NewTestEnvironment(t),
		libraries: libraries,
		sources:   map[uuid.UUID][]uuid.UUID{},
		failing:   map[uuid.UUID]error{},
	}

	running.env.ReplaceStep(running.scan.scanner.ListLibraries, func(context.Context) ([]uuid.UUID, error) {
		return running.libraries, nil
	})
	running.env.ReplaceStep(running.scan.scanner.ScanLibrary, func(_ context.Context, id uuid.UUID) error {
		running.mutex.Lock()
		defer running.mutex.Unlock()

		if err := running.failing[id]; err != nil {
			return err
		}
		running.walked = append(running.walked, id)

		return nil
	})
	running.env.ReplaceStep(running.scan.scanner.UnprobedSources, func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
		running.mutex.Lock()
		defer running.mutex.Unlock()

		return running.sources[id], nil
	})
	running.env.ReplaceStep(running.scan.scanner.ProbeSource, func(_ context.Context, id uuid.UUID) error {
		running.mutex.Lock()
		defer running.mutex.Unlock()

		if err := running.failing[id]; err != nil {
			return err
		}
		running.probed = append(running.probed, id)

		return nil
	})

	return running
}

func (r *run) needing(library uuid.UUID, count int) []uuid.UUID {
	ids := make([]uuid.UUID, 0, count)
	for range count {
		ids = append(ids, uuid.New())
	}
	r.sources[library] = ids

	return ids
}

func (r *run) fails(id uuid.UUID, because string) {
	r.failing[id] = errors.New(because)
}

func TestLibraryScan_Run(t *testing.T) {
	t.Run("a library that cannot be read does not abandon the others", func(t *testing.T) {
		first, second := uuid.New(), uuid.New()
		running := newRun(t, first, second)
		running.fails(first, "the volume is not mounted")

		if err := running.env.Run(running.scan, jobs.Options{}); err != nil {
			t.Fatalf("a failing library failed the whole scan: %v", err)
		}
		if len(running.walked) != 1 || running.walked[0] != second {
			t.Errorf("scanned = %v, want only the readable library", running.walked)
		}
	})

	t.Run("a library that could not be walked is not probed", func(t *testing.T) {
		library := uuid.New()
		running := newRun(t, library)
		running.needing(library, 3)
		running.fails(library, "the volume is not mounted")

		if err := running.env.Run(running.scan, jobs.Options{}); err != nil {
			t.Fatalf("the scan failed: %v", err)
		}
		if len(running.probed) != 0 {
			t.Errorf("probed = %v, want nothing from a library with no structure written", running.probed)
		}
	})

	t.Run("every selected file is probed, a hundred to a child run", func(t *testing.T) {
		library := uuid.New()
		running := newRun(t, library)
		selected := running.needing(library, probeChunkSize*2+5)

		if err := running.env.Run(running.scan, jobs.Options{}); err != nil {
			t.Fatalf("the scan failed: %v", err)
		}
		if len(running.probed) != len(selected) {
			t.Errorf("probed %d sources, want %d", len(running.probed), len(selected))
		}

		probed := map[uuid.UUID]bool{}
		for _, id := range running.probed {
			probed[id] = true
		}
		for _, id := range selected {
			if !probed[id] {
				t.Fatalf("source %s was selected and never probed", id)
			}
		}

		children := running.env.Children()
		if len(children) != 3 {
			t.Errorf("children = %d, want one per chunk of %d", len(children), probeChunkSize)
		}

		named := map[string]bool{}
		for _, id := range children {
			if named[id] {
				t.Fatalf("two chunks ran under the id %s", id)
			}
			named[id] = true
		}
	})

	t.Run("a file ffprobe cannot read does not abandon its chunk", func(t *testing.T) {
		library := uuid.New()
		running := newRun(t, library)
		selected := running.needing(library, 3)
		running.fails(selected[0], "ffprobe found no streams")

		if err := running.env.Run(running.scan, jobs.Options{}); err != nil {
			t.Fatalf("a failing probe failed the whole scan: %v", err)
		}
		if len(running.probed) != len(selected)-1 {
			t.Errorf("probed = %v, want the two readable files", running.probed)
		}
	})
}
