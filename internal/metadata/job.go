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
		Description: "Identifies items and fetches their metadata.",
		Run:         s.run,
	}
}

func (s *Service) run(ctx context.Context) error {
	itemID, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamItem)
	if err != nil {
		return err
	}
	if itemID != uuid.Nil {
		return s.IdentifyItem(ctx, itemID)
	}

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
