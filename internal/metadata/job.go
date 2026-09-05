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

type Identify struct {
	service *Service
}

func NewIdentify(service *Service) *Identify {
	return &Identify{service: service}
}

func (i *Identify) Name() string     { return RefreshMetadataJobID }
func (i *Identify) Category() string { return "Library" }
func (i *Identify) Description() string {
	return "Identifies items and fetches their metadata."
}

func (i *Identify) Run(ctx context.Context) error {
	scope, err := jobs.GetParam[uuid.UUID](ctx, ParamScope)
	if err != nil {
		return err
	}

	force, err := jobs.GetParam[bool](ctx, ParamForce)
	if err != nil {
		return err
	}

	return i.service.IdentifyItems(ctx, scope, force)
}
