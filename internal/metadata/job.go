package metadata

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

const (
	RefreshMetadataJobID = "RefreshMetadata"

	ParamScope = "scope"
	ParamForce = "force"
)

func (s *Service) Job() jobs.Job {
	return jobs.Job{
		Name:        RefreshMetadataJobID,
		Category:    "Library",
		Description: "Identifies items and fetches their metadata.",
		Run:         s.run,
	}
}

func (s *Service) run(ctx context.Context) error {
	scope, err := jobs.GetParam[uuid.UUID](ctx, ParamScope)
	if err != nil {
		return err
	}

	force, err := jobs.GetParam[bool](ctx, ParamForce)
	if err != nil {
		return err
	}

	return s.IdentifyItems(ctx, scope, force)
}
