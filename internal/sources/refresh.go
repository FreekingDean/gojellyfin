package sources

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	librarysourcemodel "github.com/FreekingDean/gojellyfin/internal/store/librarysource"
)

func (s *Service) RefreshJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.RefreshLibrarySource,
		Category:    "Library",
		Description: "Reads one Sonarr or Radarr instance for one library.",
		Run:         s.refresh,
	}
}

func (s *Service) refresh(ctx context.Context) error {
	libraryID, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamLibrary)
	if err != nil {
		return err
	}

	sourceID, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamSource)
	if err != nil {
		return err
	}

	binding, err := s.binding(ctx, libraryID, sourceID)
	if err != nil {
		return err
	}

	titles, err := s.Titles(ctx, binding)
	if err != nil {
		return err
	}

	paths := make([]string, 0, len(titles))
	keys := make([]string, 0, len(titles))
	for _, title := range titles {
		if title.Tagged {
			keys = append(keys, title.Key)
		}
		for _, file := range title.Files {
			paths = append(paths, file.Path)
		}
	}

	if _, err := s.items.DeleteSourcesNotInPaths(ctx, sourceID, paths); err != nil {
		return err
	}
	if _, err := s.items.DeleteMembershipNotInKeys(ctx, libraryID, sourceID, keys); err != nil {
		return err
	}

	for _, title := range titles {
		jobs.Heartbeat(ctx, title.Name)

		if err := jobs.Enqueue(ctx, jobs.RefreshItem,
			jobs.With(jobs.ParamItem, scanned(title)),
			jobs.With(jobs.ParamLibrary, libraryID),
			jobs.With(jobs.ParamSource, sourceID),
		); err != nil {
			log.Printf("failed to enqueue %s: %v", title.Key, err)
		}
	}

	s.activity.Record(ctx, activity.Entry{
		Name:          fmt.Sprintf("%s read", binding.Source.Name),
		Kind:          activity.KindLibraryScanCompleted,
		ShortOverview: fmt.Sprintf("%d titles, %d files", len(titles), len(paths)),
		Severity:      activity.SeverityInformation,
	})

	return nil
}

func (s *Service) binding(ctx context.Context, libraryID, sourceID uuid.UUID) (Binding, error) {
	record, err := s.store.LibrarySource.Query().
		Where(
			librarysourcemodel.LibraryID(libraryID),
			librarysourcemodel.SourceID(sourceID),
		).
		WithSource().
		Only(ctx)
	if err != nil {
		return Binding{}, fmt.Errorf("failed to find the binding: %w", err)
	}
	if record.Edges.Source == nil {
		return Binding{}, fmt.Errorf("binding %s names no source", record.ID)
	}

	return Binding{Source: *record.Edges.Source, Library: *record}, nil
}

func scanned(title Title) items.Scanned {
	files := make([]items.ScannedFile, 0, len(title.Files))
	for _, file := range title.Files {
		files = append(files, items.ScannedFile{Path: file.Path, DateModified: file.DateModified})
	}

	return items.Scanned{
		Key:         title.Key,
		ParentKey:   title.ParentKey,
		Tagged:      title.Tagged,
		Kind:        title.Kind,
		Name:        title.Name,
		SortName:    title.SortName,
		Year:        title.Year,
		Index:       title.Index,
		ParentIndex: title.ParentIndex,
		Files:       files,
	}
}
