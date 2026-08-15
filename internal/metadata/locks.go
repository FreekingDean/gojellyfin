package metadata

import "github.com/FreekingDean/gojellyfin/internal/items"

// A manual edit surviving a refresh is what these columns are for, so a locked
// field is dropped from what the provider writes rather than merged over. The
// names are Jellyfin's MetadataField values; the ones missing here cover
// columns no provider of ours writes yet.
var locks = map[string]func(*items.Metadata){
	"Name": func(metadata *items.Metadata) {
		metadata.Name = nil
		metadata.OriginalTitle = nil
		metadata.SortName = nil
	},
	"Overview": func(metadata *items.Metadata) {
		metadata.Overview = nil
		metadata.Taglines = nil
	},
	"OfficialRating": func(metadata *items.Metadata) {
		metadata.OfficialRating = nil
	},
	"ProductionLocations": func(metadata *items.Metadata) {
		metadata.ProductionLocations = nil
	},
	"Tags": func(metadata *items.Metadata) {
		metadata.Tags = nil
	},
}

// The ids are identity rather than metadata and Jellyfin has no lock for them,
// so they survive every lock short of LockData, which keeps the item out of the
// batch altogether.
func withoutLocked(metadata items.Metadata, locked []string) items.Metadata {
	for _, field := range locked {
		if strip, known := locks[field]; known {
			strip(&metadata)
		}
	}

	return metadata
}
