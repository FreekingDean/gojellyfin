package collage

import (
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

const RefreshLibraryImagesJobID = "RefreshLibraryImages"

type LibraryImages struct {
	service *Service
}

func NewLibraryImages(service *Service) *LibraryImages {
	return &LibraryImages{service: service}
}

func (l *LibraryImages) Name() string     { return RefreshLibraryImagesJobID }
func (l *LibraryImages) Category() string { return "Library" }
func (l *LibraryImages) Description() string {
	return "Builds a poster collage for each library from the artwork its titles carry."
}

func (l *LibraryImages) Steps() []any {
	return []any{l.service.Libraries, l.service.BuildLibraryImage}
}

func (l *LibraryImages) Children() []any { return nil }

func (l *LibraryImages) Run(ctx jobs.Context, _ jobs.Options) error {
	var catalogued []uuid.UUID
	if err := jobs.Step(ctx, l.service.Libraries).Get(&catalogued); err != nil {
		return err
	}

	builds := make([]jobs.Future, 0, len(catalogued))
	for _, id := range catalogued {
		builds = append(builds, jobs.Step(ctx, l.service.BuildLibraryImage, id))
	}

	for index, build := range builds {
		if err := build.Get(nil); err != nil {
			jobs.Logf(ctx, "library image failed", "library", catalogued[index], "error", err)
		}
	}

	return nil
}
