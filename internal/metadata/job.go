package metadata

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

func (s *Service) Job() jobs.Job {
	return jobs.Job{
		Name:        jobs.RefreshMetadata,
		Category:    "Library",
		Description: "Identifies every item nothing has identified yet.",
		Startable:   true,
		Run:         s.runBatch,
	}
}

func (s *Service) ItemJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.RefreshItemMetadata,
		Category:    "Library",
		Description: "Identifies one item.",
		Run:         s.runOne,
	}
}

func (s *Service) runBatch(ctx context.Context) error {
	scope, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamScope)
	if err != nil {
		return err
	}

	force, err := jobs.GetParam[bool](ctx, jobs.ParamForce)
	if err != nil {
		return err
	}

	return s.IdentifyItems(ctx, scope, force)
}

func (s *Service) runOne(ctx context.Context) error {
	itemID, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamItem)
	if err != nil {
		return err
	}

	return s.IdentifyItem(ctx, itemID)
}
