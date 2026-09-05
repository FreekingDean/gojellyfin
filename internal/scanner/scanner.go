package scanner

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type Scanner struct {
	items      *items.Service
	libraries  *libraries.Service
	sources    *sources.Service
	filesystem *filesystem.Service
	ffmpeg     *ffmpeg.FFMpeg
	activity   *activity.Service
}

func New(
	items *items.Service,
	libraries *libraries.Service,
	sources *sources.Service,
	filesystem *filesystem.Service,
	ffmpeg *ffmpeg.FFMpeg,
	activity *activity.Service,
) *Scanner {
	return &Scanner{
		items:      items,
		libraries:  libraries,
		sources:    sources,
		filesystem: filesystem,
		ffmpeg:     ffmpeg,
		activity:   activity,
	}
}

type seen struct {
	keys       []string
	paths      map[uuid.UUID][]string
	members    map[uuid.UUID][]uuid.UUID
	unreadable int
}

func found() *seen {
	return &seen{
		paths:   map[uuid.UUID][]string{},
		members: map[uuid.UUID][]uuid.UUID{},
	}
}

func (s *seen) title(source uuid.UUID, item *items.Item) {
	s.keys = append(s.keys, item.Key)
	s.members[source] = append(s.members[source], item.ID)
}

func (s *seen) file(source uuid.UUID, path string) {
	s.paths[source] = append(s.paths[source], path)
}

func (s *seen) files() int {
	total := 0
	for _, paths := range s.paths {
		total += len(paths)
	}

	return total
}

func (s *seen) skip(name string, err error) {
	s.unreadable++
	log.Printf("skipping %s: %v", name, err)
}

func (s *seen) complete() bool {
	return s.unreadable == 0
}

func (s *Scanner) scanLibrary(ctx context.Context, library *libraries.Library) error {
	found := found()

	bindings, err := s.sources.BindingsFor(ctx, library.ID)
	if err != nil {
		return err
	}
	if len(bindings) == 0 {
		log.Printf("not scanning %s: no source is bound to it", library.Name)

		return nil
	}

	for _, binding := range bindings {
		titles, err := s.sources.Titles(ctx, binding)
		if err != nil {
			found.skip(binding.Source.Name, err)

			continue
		}

		for _, title := range titles {
			if err := s.saveTitle(ctx, library, binding.Source.ID, nil, "", title, found); err != nil {
				return err
			}
		}

		members := found.members[binding.Source.ID]
		if err := s.items.SaveMembership(ctx, library.ID, binding.Source.ID, members); err != nil {
			return err
		}
	}

	log.Printf("scanned %s: %d items, %d files", library.Name, len(found.keys), found.files())

	if !found.complete() {
		log.Printf("not sweeping %s: %d could not be read", library.Name, found.unreadable)

		return nil
	}

	for source, members := range found.members {
		if err := s.items.DeleteMembershipNotIn(ctx, library.ID, source, members); err != nil {
			return err
		}
	}

	for source, paths := range found.paths {
		if err := s.items.DeleteSourcesNotInPaths(ctx, source, paths); err != nil {
			return err
		}
	}

	s.activity.Record(ctx, activity.Event{
		Name:          fmt.Sprintf("%s scan completed", library.Name),
		Kind:          activity.KindLibraryScanCompleted,
		ShortOverview: fmt.Sprintf("%d items, %d files", len(found.keys), found.files()),
		Severity:      activity.SeverityInformation,
	})

	return nil
}

func (s *Scanner) saveTitle(
	ctx context.Context,
	library *libraries.Library,
	source uuid.UUID,
	parent *uuid.UUID,
	slug string,
	title sources.Title,
	found *seen,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	scanned := items.Scanned{
		ParentID:     parent,
		Kind:         title.Kind,
		Name:         title.Name,
		SortName:     sortName(title.Name),
		DateModified: modified(title),
	}

	switch title.Kind {
	case itemmodal.KindMovie:
		scanned.Key = movieKey(title.Name, title.Year)
		scanned.ProductionYear = title.Year
	case itemmodal.KindSeries:
		slug = titleSlug(title.Name, title.Year)
		scanned.Key = seriesKey(slug)
		scanned.ProductionYear = title.Year
	case itemmodal.KindSeason:
		scanned.Key = seasonKey(slug, title.Index)
		scanned.Name = seasonName(title.Index)
		scanned.SortName = seasonSortName(title.Index)
		scanned.IndexNumber = title.Index
	case itemmodal.KindEpisode:
		scanned.Name = episodeName(title.Name, title.Index)
		scanned.SortName = sortName(scanned.Name)
		scanned.Key = episodeKey(slug, title.ParentIndex, title.Index, scanned.Name)
		scanned.IndexNumber = title.Index
		scanned.ParentIndexNumber = title.ParentIndex
	default:
		return fmt.Errorf("unsupported kind %s", title.Kind)
	}

	item, err := s.items.SaveScanned(ctx, scanned)
	if err != nil {
		return err
	}
	found.title(source, item)

	for _, file := range title.Files {
		if err := s.saveFile(ctx, source, item, file, found); err != nil {
			return err
		}
	}

	for _, child := range title.Children {
		if err := s.saveTitle(ctx, library, source, &item.ID, slug, child, found); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scanner) saveFile(
	ctx context.Context,
	sourceID uuid.UUID,
	item *items.Item,
	file sources.File,
	found *seen,
) error {
	jobs.Heartbeat(ctx, file.Path)
	found.file(sourceID, file.Path)

	source, err := s.items.SaveSource(ctx, items.ScannedSource{
		SourceID:     sourceID,
		ItemID:       item.ID,
		Path:         file.Path,
		Name:         filepath.Base(file.Path),
		DateModified: file.DateModified,
	})
	if err != nil {
		return err
	}

	if err := s.scanSubtitles(ctx, item.ID, source); err != nil {
		log.Printf("subtitles %s: %v", file.Path, err)
	}

	return nil
}

func modified(title sources.Title) time.Time {
	newest := time.Time{}
	for _, file := range title.Files {
		if file.DateModified.After(newest) {
			newest = file.DateModified
		}
	}

	for _, child := range title.Children {
		if when := modified(child); when.After(newest) {
			newest = when
		}
	}

	return newest
}
