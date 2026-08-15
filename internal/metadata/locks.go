package metadata

import "github.com/FreekingDean/gojellyfin/internal/items"

func stripLockedFields(metadata *items.Metadata, locked []string) {
	locks := map[string]func(){
		"Name": func() {
			metadata.Name = nil
			metadata.OriginalTitle = nil
			metadata.SortName = nil
		},
		"Overview": func() {
			metadata.Overview = nil
			metadata.Taglines = nil
		},
		"OfficialRating": func() {
			metadata.OfficialRating = nil
		},
		"ProductionLocations": func() {
			metadata.ProductionLocations = nil
		},
		"Tags": func() {
			metadata.Tags = nil
		},
	}

	for _, field := range locked {
		if strip, known := locks[field]; known {
			strip()
		}
	}
}
