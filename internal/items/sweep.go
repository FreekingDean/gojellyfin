package items

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (s *Service) SweepJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.SweepItems,
		Category:    "Library",
		Description: "Removes titles no source reports a file for any more.",
		Startable:   true,
		Run:         s.sweep,
	}
}

func (s *Service) sweep(ctx context.Context) error {
	everything, err := s.store.Item.Query().Where(itemmodal.DeletedAtIsNil()).IDs(ctx)
	if err != nil {
		return err
	}

	return s.SweepUnreachable(ctx, everything)
}
