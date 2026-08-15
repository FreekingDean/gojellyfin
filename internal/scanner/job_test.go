package scanner

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

func TestLibraryScanScansEveryLibraryDespiteAFailure(t *testing.T) {
	env := jobs.NewTestEnvironment(t)
	scan := NewLibraryScan(&Scanner{})

	first, second := uuid.New(), uuid.New()
	scanned := make([]uuid.UUID, 0, 2)

	env.ReplaceStep(scan.scanner.ListLibraries, func(context.Context) ([]uuid.UUID, error) {
		return []uuid.UUID{first, second}, nil
	})
	env.ReplaceStep(scan.scanner.ScanLibrary, func(_ context.Context, id uuid.UUID) error {
		if id == first {
			return errors.New("the volume is not mounted")
		}
		scanned = append(scanned, id)

		return nil
	})

	if err := env.Run(scan); err != nil {
		t.Fatalf("a failing library failed the whole scan: %v", err)
	}
	if len(scanned) != 1 || scanned[0] != second {
		t.Errorf("scanned = %v, want only the readable library", scanned)
	}
}
