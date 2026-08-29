package metadata

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

const downloadLimit = 16 << 20

func (s *Service) saveArtwork(ctx context.Context, pendingItem *items.Item, references []items.RemoteImage) {
	if len(references) == 0 {
		return
	}

	existing, err := s.items.Images(ctx, pendingItem.ID)
	if err != nil {
		log.Printf("metadata %s artwork: %v", pendingItem.Name, err)

		return
	}

	held := map[items.ImageKind]*items.Image{}
	for _, image := range existing {
		if image.Index == 0 {
			held[image.Kind] = image
		}
	}

	for _, reference := range references {
		if err := ctx.Err(); err != nil {
			return
		}

		jobs.Heartbeat(ctx, pendingItem.Name, string(reference.Kind))

		if err := s.storeArtwork(ctx, pendingItem.ID, held[reference.Kind], reference); err != nil {
			log.Printf("metadata %s %s artwork: %v", pendingItem.Name, reference.Kind, err)
		}
	}
}

func (s *Service) storeArtwork(ctx context.Context, itemID uuid.UUID, held *items.Image, reference items.RemoteImage) error {
	if held != nil && held.Source == items.ImageSourceLocal {
		return nil
	}

	sum := sha1.Sum([]byte(reference.URL))
	tag := hex.EncodeToString(sum[:])
	suffix := path.Ext(reference.URL)
	if suffix == "" {
		suffix = ".jpg"
	}
	key := fmt.Sprintf("items/%s/%s/%s%s", itemID, reference.Kind, tag, suffix)

	if held != nil && held.Path == key {
		return nil
	}

	body, err := s.fetchArtwork(ctx, reference.URL)
	if err != nil {
		return err
	}

	if err := s.artwork.Put(ctx, key, bytes.NewReader(body)); err != nil {
		return err
	}

	saved := items.Artwork{Kind: reference.Kind, Path: key, Tag: tag, Size: int64(len(body))}
	if err := s.items.SaveDownloadedImage(ctx, itemID, saved); err != nil {
		return err
	}

	if held != nil {
		return s.artwork.Delete(ctx, held.Path)
	}

	return nil
}

func (s *Service) fetchArtwork(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ask for %s: %w", url, err)
	}

	response, err := s.downloads.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", url, response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, downloadLimit+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", url, err)
	}
	if len(body) > downloadLimit {
		return nil, fmt.Errorf("%s is larger than %d bytes", url, downloadLimit)
	}

	return body, nil
}
