package metadata

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

// Declared here rather than beside its implementation so that nothing outside
// this package names a provider: which service is asked is wiring, and a caller
// sees only that metadata can be fetched. A miss is `false` rather than an
// error, because a title a provider does not carry is not a failed run.
type Provider interface {
	Enabled() bool
	Movie(ctx context.Context, name string, year *int32) (items.Metadata, bool, error)
	Series(ctx context.Context, name string, year *int32) (items.Metadata, bool, error)
	Episode(ctx context.Context, series map[string]string, season, episode int32) (items.Metadata, bool, error)
}
