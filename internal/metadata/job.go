package metadata

import "github.com/FreekingDean/gojellyfin/internal/jobs"

const RefreshMetadataJobID = "RefreshMetadata"

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

func (i *Identify) Steps() []any {
	return []any{i.service.IdentifyItems}
}

func (i *Identify) Children() []any { return nil }

func (i *Identify) Run(ctx jobs.Context) error {
	return jobs.Step(ctx, i.service.IdentifyItems).Get(nil)
}
