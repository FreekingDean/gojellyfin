package scanner

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
)

var imageExtensions = []string{".jpg", ".jpeg", ".png", ".webp"}

var artworkNames = map[items.ImageKind][]string{
	imagemodal.KindPrimary:  {"poster", "folder", "cover", "default", "movie", "show"},
	imagemodal.KindBackdrop: {"fanart", "backdrop", "background", "art"},
	imagemodal.KindThumb:    {"thumb", "landscape"},
	imagemodal.KindLogo:     {"logo", "clearlogo"},
	imagemodal.KindBanner:   {"banner"},
}

func (s *Scanner) scanArtwork(ctx context.Context, itemID uuid.UUID, directory, base string, folder bool, found *seen) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	for kind, names := range artworkNames {
		stems := make([]string, 0, len(names)*2+1)
		if base != "" {
			for _, name := range names {
				stems = append(stems, strings.ToLower(base)+"-"+name)
			}
			if kind == imagemodal.KindPrimary {
				stems = append(stems, strings.ToLower(base))
			}
		}
		if folder {
			stems = append(stems, names...)
		}

		match := findArtwork(entries, directory, stems)
		if match == "" {
			continue
		}

		image, err := describeImage(kind, match)
		if err != nil {
			continue
		}
		if err := s.items.SaveImage(ctx, itemID, image); err != nil {
			return err
		}
		found.image(image.Path)
	}

	return nil
}

func findArtwork(entries []os.DirEntry, directory string, stems []string) string {
	for _, stem := range stems {
		for _, entry := range entries {
			if entry.IsDir() || !isImage(entry.Name()) {
				continue
			}
			if strings.ToLower(stripExtension(entry.Name())) != stem {
				continue
			}

			return filepath.Join(directory, entry.Name())
		}
	}

	return ""
}

func describeImage(kind items.ImageKind, path string) (items.Artwork, error) {
	info, err := os.Stat(path)
	if err != nil {
		return items.Artwork{}, err
	}

	width, height := imageSize(path)

	return items.Artwork{
		Kind:   kind,
		Path:   path,
		Tag:    imageTag(path, info.ModTime().UnixNano(), info.Size()),
		Width:  int32(width),
		Height: int32(height),
		Size:   info.Size(),
	}, nil
}

func imageTag(path string, modified, size int64) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s:%d:%d", path, modified, size)))

	return hex.EncodeToString(sum[:])
}

func imageSize(path string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = file.Close() }()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0
	}

	return config.Width, config.Height
}

func isImage(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	for _, candidate := range imageExtensions {
		if extension == candidate {
			return true
		}
	}

	return false
}
