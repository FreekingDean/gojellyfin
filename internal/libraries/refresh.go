package libraries

import (
	"context"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
)

func (s *Service) RefreshLibrariesJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.RefreshLibraries,
		Category:    "Library",
		Description: "Refreshes every library from its Sonarr and Radarr sources.",
		Startable:   true,
		Run:         s.refreshAll,
	}
}

func (s *Service) RefreshLibraryJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.RefreshLibrary,
		Category:    "Library",
		Description: "Refreshes one library from each source bound to it.",
		Run:         s.refreshOne,
	}
}

func (s *Service) refreshAll(ctx context.Context) error {
	libraries, err := s.ListLibraries(ctx)
	if err != nil {
		return err
	}

	for _, library := range libraries {
		jobs.Heartbeat(ctx, library.Name)

		if err := jobs.Enqueue(ctx, jobs.RefreshLibrary, jobs.With(jobs.ParamLibrary, library.ID)); err != nil {
			log.Printf("failed to enqueue %s: %v", library.Name, err)
		}
	}

	return nil
}

func (s *Service) refreshOne(ctx context.Context) error {
	libraryID, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamLibrary)
	if err != nil {
		return err
	}

	library, err := s.Library(ctx, libraryID)
	if err != nil {
		return err
	}

	bindings, err := s.sources.BindingsFor(ctx, libraryID)
	if err != nil {
		return err
	}
	if len(bindings) == 0 {
		log.Printf("not refreshing %s: no source is bound to it", library.Name)

		return nil
	}

	for _, binding := range bindings {
		jobs.Heartbeat(ctx, binding.Source.Name)

		if err := jobs.Enqueue(ctx, jobs.RefreshLibrarySource,
			jobs.With(jobs.ParamLibrary, libraryID),
			jobs.With(jobs.ParamSource, binding.Source.ID),
		); err != nil {
			log.Printf("failed to enqueue %s: %v", binding.Source.Name, err)
		}
	}

	return nil
}
