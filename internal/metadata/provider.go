package metadata

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

type Provider interface {
	Enabled() bool
	Movie(ctx context.Context, name string, year *int32) (items.Metadata, bool, error)
	Series(ctx context.Context, name string, year *int32) (items.Metadata, bool, error)
	Episode(ctx context.Context, series map[string]string, season, episode int32) (items.Metadata, bool, error)
}
