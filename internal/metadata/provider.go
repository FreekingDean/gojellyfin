package metadata

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

type Provider interface {
	Enabled() bool
	Movie(ctx context.Context, tmdbID int) (items.Metadata, bool, error)
	Series(ctx context.Context, tmdbID int) (items.Metadata, bool, error)
	Season(ctx context.Context, series int, season int32) (items.Metadata, bool, error)
	Episode(ctx context.Context, series int, season, episode int32) (items.Metadata, bool, error)
}
