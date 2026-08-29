package collage

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/artwork"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

const posterLimit = 16 << 20

type Service struct {
	libraries  *libraries.Service
	items      *items.Service
	filesystem *filesystem.Service
	artwork    artwork.Store
}

func New(catalogue *libraries.Service, records *items.Service, files *filesystem.Service, stored artwork.Store) *Service {
	return &Service{libraries: catalogue, items: records, filesystem: files, artwork: stored}
}

func (s *Service) Libraries(ctx context.Context) ([]uuid.UUID, error) {
	catalogued, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(catalogued))
	for _, library := range catalogued {
		ids = append(ids, library.ID)
	}

	return ids, nil
}

func (s *Service) BuildLibraryImage(ctx context.Context, id uuid.UUID) error {
	library, err := s.libraries.Library(ctx, id)
	if err != nil {
		return err
	}

	jobs.Heartbeat(ctx, library.Name)

	chosen, err := s.items.LibraryPosters(ctx, id, cells)
	if err != nil {
		return err
	}

	tag := fingerprint(chosen)
	if tag == library.ImageTag {
		return nil
	}
	if tag == "" {
		return s.replace(ctx, library, "", nil)
	}

	posters := make([]image.Image, 0, len(chosen))
	for _, record := range chosen {
		poster, err := s.decode(ctx, record)
		if err != nil {
			log.Printf("collage %s poster: %v", library.Name, err)

			continue
		}
		posters = append(posters, poster)
	}
	if len(posters) == 0 {
		return nil
	}

	body, err := compose(posters)
	if err != nil {
		return fmt.Errorf("failed to compose the %s collage: %w", library.Name, err)
	}

	return s.replace(ctx, library, tag, body)
}

func (s *Service) replace(ctx context.Context, library *libraries.Library, tag string, body []byte) error {
	if tag != "" {
		if err := s.artwork.Put(ctx, libraries.ImageKey(library.ID, tag), bytes.NewReader(body)); err != nil {
			return err
		}
	}

	if err := s.libraries.SetImageTag(ctx, library.ID, tag); err != nil {
		return err
	}

	if library.ImageTag == "" {
		return nil
	}

	return s.artwork.Delete(ctx, libraries.ImageKey(library.ID, library.ImageTag))
}

func (s *Service) decode(ctx context.Context, record *items.Image) (image.Image, error) {
	body, err := s.open(ctx, record)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()

	poster, _, err := image.Decode(io.LimitReader(body, posterLimit))
	if err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", record.Path, err)
	}

	return poster, nil
}

func (s *Service) open(ctx context.Context, record *items.Image) (io.ReadCloser, error) {
	if record.Source == items.ImageSourceLocal {
		body, _, err := s.filesystem.Open(ctx, record.Path)

		return body, err
	}

	body, _, found, err := s.artwork.Open(ctx, record.Path)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%s is not stored", record.Path)
	}

	return body, nil
}

func fingerprint(chosen []*items.Image) string {
	if len(chosen) == 0 {
		return ""
	}

	sum := sha1.New()
	for _, record := range chosen {
		fmt.Fprintf(sum, "%s:%s\n", record.ID, record.Tag)
	}

	return hex.EncodeToString(sum.Sum(nil))
}
