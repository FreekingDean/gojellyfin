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

	// Registers the decoders image.DecodeConfig needs to read artwork
	// dimensions; without them every image reports 0x0.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

var imageExtensions = []string{".jpg", ".jpeg", ".png", ".webp"}

// Artwork sits beside the media under conventional names. Order matters: the
// first match for a type wins.
var artworkNames = map[string][]string{
	"Primary":  {"poster", "folder", "cover", "default", "movie", "show"},
	"Backdrop": {"fanart", "backdrop", "background", "art"},
	"Thumb":    {"thumb", "landscape"},
	"Logo":     {"logo", "clearlogo"},
	"Banner":   {"banner"},
}

// scanArtwork records the images belonging to one item. A folder item looks
// inside itself; a file item looks beside itself for art named after the file.
func (s *Scanner) scanArtwork(ctx context.Context, itemID uuid.UUID, path string, isFolder bool) error {
	directory, base := path, ""
	if !isFolder {
		directory, base = filepath.Dir(path), stripExtension(filepath.Base(path))
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	found := make([]items.ItemImage, 0)
	for imageType, names := range artworkNames {
		match := findArtwork(entries, directory, base, names)
		if match == "" {
			continue
		}

		image, err := describeImage(itemID, imageType, match)
		if err != nil {
			continue
		}
		found = append(found, image)
	}

	return s.items.ReplaceImages(ctx, itemID, found)
}

func findArtwork(entries []os.DirEntry, directory, base string, names []string) string {
	for _, name := range names {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			candidate := strings.ToLower(stripExtension(entry.Name()))
			if !isImage(entry.Name()) {
				continue
			}

			// "poster.jpg" beside a folder, or "Movie (1982)-poster.jpg"
			// beside the file it belongs to.
			switch {
			case base == "" && candidate == name:
			case base != "" && candidate == strings.ToLower(base)+"-"+name:
			case base != "" && candidate == strings.ToLower(base) && name == "poster":
			default:
				continue
			}

			return filepath.Join(directory, entry.Name())
		}
	}

	return ""
}

func describeImage(itemID uuid.UUID, imageType, path string) (items.ItemImage, error) {
	info, err := os.Stat(path)
	if err != nil {
		return items.ItemImage{}, err
	}

	width, height := imageSize(path)

	return items.ItemImage{
		ItemID: itemID,
		Type:   imageType,
		Path:   path,
		Tag:    imageTag(path, info.ModTime().UnixNano(), info.Size()),
		Width:  int32(width),
		Height: int32(height),
		Size:   info.Size(),
	}, nil
}

// The tag is what the client caches against, so it must change when the file
// does.
func imageTag(path string, modified, size int64) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s:%d:%d", path, modified, size)))

	return hex.EncodeToString(sum[:])
}

func imageSize(path string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

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
