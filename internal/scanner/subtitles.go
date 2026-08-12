package scanner

import (
	"context"
	"path/filepath"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
)

func (s *Scanner) scanSubtitles(ctx context.Context, item *items.Item) error {
	found, err := subtitlesBeside(ctx, s.filesystem, item.Path)
	if err != nil {
		return err
	}

	return s.items.ReplaceExternalSubtitles(ctx, item, found)
}

func subtitlesBeside(ctx context.Context, files *filesystem.Service, path string) ([]items.ExternalSubtitle, error) {
	directory := filepath.Dir(path)
	base := stripExtension(filepath.Base(path))

	entries, err := files.List(ctx, directory)
	if err != nil {
		return nil, err
	}

	found := make([]items.ExternalSubtitle, 0)
	for _, entry := range entries {
		if entry.Dir {
			continue
		}

		subtitle, ok := parseSubtitle(base, entry.Name)
		if !ok {
			continue
		}
		subtitle.Path = filepath.Join(directory, entry.Name)
		found = append(found, subtitle)
	}

	return found, nil
}
