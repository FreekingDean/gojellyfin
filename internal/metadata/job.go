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
		Description: "Identifies every item nothing has identified yet, and fetches artwork for any item that has none.",
		Startable:   true,
		Queue:       jobs.MetadataQueue,
		Run:         s.runBatch,
		Outstanding: s.outstanding,
	}
}

func (s *Service) outstanding(ctx context.Context) (int, error) {
	if !s.provider.Enabled() {
		return 0, nil
	}

	pending, err := s.items.ItemsNeedingMetadata(ctx, identifiable, false, uuid.Nil)

	return len(pending), err
}

func (s *Service) ItemJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.RefreshItemMetadata,
		Category:    "Library",
		Description: "Identifies one item.",
		Queue:       jobs.MetadataQueue,
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
