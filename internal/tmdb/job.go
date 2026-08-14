package tmdb

import "github.com/FreekingDean/gojellyfin/internal/jobs"

const RefreshMetadataJobID = "RefreshMetadata"

type Identify struct {
	provider *Provider
}

func NewIdentify(provider *Provider) *Identify {
	return &Identify{provider: provider}
}

func (i *Identify) Name() string     { return RefreshMetadataJobID }
func (i *Identify) Category() string { return "Library" }
func (i *Identify) Description() string {
	return "Identifies items against TMDB and fetches their metadata."
}

func (i *Identify) Steps() []any {
	return []any{i.provider.IdentifyItems}
}

func (i *Identify) Run(ctx jobs.Context) error {
	return jobs.Step(ctx, i.provider.IdentifyItems).Get(nil)
}
