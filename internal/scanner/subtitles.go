package scanner

import (
	"context"
	"os"
	"path/filepath"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

// scanSubtitles records the subtitle files sitting beside one media file. It
// runs after the probe, which recreates the media source and takes the
// container's own streams with it.
func (s *Scanner) scanSubtitles(ctx context.Context, item *items.Item) error {
	directory := filepath.Dir(item.Path)
	base := stripExtension(filepath.Base(item.Path))

	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	found := make([]items.ExternalSubtitle, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		subtitle, ok := parseSubtitle(base, entry.Name())
		if !ok {
			continue
		}
		subtitle.Path = filepath.Join(directory, entry.Name())
		found = append(found, subtitle)
	}

	return s.items.ReplaceExternalSubtitles(ctx, item, found)
}
