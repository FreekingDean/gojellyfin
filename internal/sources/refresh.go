package sources

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/store"
	librarysourcemodel "github.com/FreekingDean/gojellyfin/internal/store/librarysource"
)

const (
	recordTimeout      = 5 * time.Second
	enqueueConcurrency = 32
)

func (s *Service) RefreshJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.RefreshLibrarySource,
		Category:    "Library",
		Description: "Reads one Sonarr or Radarr instance for one library.",
		Queue:       jobs.RefreshQueue,
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

	readErr := s.read(ctx, binding)
	if err := s.recordSync(ctx, binding.Library.ID, readErr); err != nil {
		log.Printf("failed to record the sync of %s: %v", binding.Source.Name, err)
	}

	return readErr
}

func (s *Service) recordSync(ctx context.Context, bindingID uuid.UUID, readErr error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()

	update := s.store.LibrarySource.UpdateOneID(bindingID)
	if readErr != nil {
		return update.SetLastError(readErr.Error()).Exec(ctx)
	}

	return update.SetLastSyncedAt(time.Now()).SetLastError("").Exec(ctx)
}

func (s *Service) read(ctx context.Context, binding Binding) error {
	libraryID, sourceID := binding.Library.LibraryID, binding.Source.ID

	if err := s.contact(ctx, binding.Source); err != nil {
		return err
	}
	if binding.Source.WebhookRegisteredAt == nil {
		s.hookQuietly(ctx, sourceID)
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

	enqueuing, enqueueCtx := errgroup.WithContext(ctx)
	enqueuing.SetLimit(enqueueConcurrency)

	for _, title := range titles {
		jobs.Heartbeat(ctx, title.Name)

		enqueuing.Go(func() error {
			if err := jobs.Enqueue(enqueueCtx, jobs.RefreshItem,
				jobs.With(jobs.ParamItem, scanned(title)),
				jobs.With(jobs.ParamLibrary, libraryID),
				jobs.With(jobs.ParamSource, sourceID),
			); err != nil {
				log.Printf("failed to enqueue %s: %v", title.Key, err)
			}

			return enqueueCtx.Err()
		})
	}

	if err := enqueuing.Wait(); err != nil {
		return err
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
		WithSource(func(query *store.SourceQuery) { query.WithPaths() }).
		OnlyModel(ctx)
	if err != nil {
		return Binding{}, fmt.Errorf("failed to find the binding: %w", err)
	}
	if record.Source == nil {
		return Binding{}, fmt.Errorf("binding %s names no source", record.ID)
	}

	return Binding{Source: *record.Source, Library: *record}, nil
}

func scanned(title Title) items.Scanned {
	files := make([]items.ScannedFile, 0, len(title.Files))
	for _, file := range title.Files {
		files = append(files, items.ScannedFile{
			Path:           file.Path,
			DateModified:   file.DateModified,
			ExternalID:     file.ExternalID,
			ExternalFileID: file.ExternalFileID,
		})
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
