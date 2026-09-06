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

func (s *seen) reached(source uuid.UUID) {
	if _, held := s.paths[source]; !held {
		s.paths[source] = []string{}
	}
	if _, held := s.members[source]; !held {
		s.members[source] = []uuid.UUID{}
	}
}

func (s *seen) unreached(source uuid.UUID) {
	delete(s.paths, source)
	delete(s.members, source)
}

func (s *seen) title(source uuid.UUID, item *items.Item, tagged bool) {
	s.keys = append(s.keys, item.Key)
	if tagged {
		s.members[source] = append(s.members[source], item.ID)
	}
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

func (s *Scanner) scanLibrary(ctx context.Context, library *libraries.Library) ([]uuid.UUID, error) {
	found := found()

	bindings, err := s.sources.BindingsFor(ctx, library.ID)
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		log.Printf("not scanning %s: no source is bound to it", library.Name)

		return nil, nil
	}

	for _, binding := range bindings {
		found.reached(binding.Source.ID)

		titles, err := s.sources.Titles(ctx, binding)
		if err != nil {
			found.skip(binding.Source.Name, err)
			found.unreached(binding.Source.ID)

			continue
		}

		for _, title := range titles {
			if err := s.saveTitle(ctx, library, binding.Source.ID, nil, "", title, title.Tagged, found); err != nil {
				return nil, err
			}
		}

		members := found.members[binding.Source.ID]
		if err := s.items.SaveMembership(ctx, library.ID, binding.Source.ID, members); err != nil {
			return nil, err
		}
	}

	log.Printf("scanned %s: %d items, %d files", library.Name, len(found.keys), found.files())

	if !found.complete() {
		log.Printf("not sweeping %s: %d could not be read", library.Name, found.unreadable)

		return nil, nil
	}

	disturbed := make([]uuid.UUID, 0)

	for source, members := range found.members {
		dropped, err := s.items.DeleteMembershipNotIn(ctx, library.ID, source, members)
		if err != nil {
			return nil, err
		}
		disturbed = append(disturbed, dropped...)
	}

	for source, paths := range found.paths {
		dropped, err := s.items.DeleteSourcesNotInPaths(ctx, source, paths)
		if err != nil {
			return nil, err
		}
		disturbed = append(disturbed, dropped...)
	}

	s.activity.Record(ctx, activity.Entry{
		Name:          fmt.Sprintf("%s scan completed", library.Name),
		Kind:          activity.KindLibraryScanCompleted,
		ShortOverview: fmt.Sprintf("%d items, %d files", len(found.keys), found.files()),
		Severity:      activity.SeverityInformation,
	})

	return disturbed, nil
}

func (s *Scanner) saveTitle(
	ctx context.Context,
	library *libraries.Library,
	source uuid.UUID,
	parent *uuid.UUID,
	slug string,
	title sources.Title,
	tagged bool,
	found *seen,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	scanned := items.Item{
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
	found.title(source, item, tagged)

	for _, file := range title.Files {
		if err := s.saveFile(ctx, source, item, file, found); err != nil {
			return err
		}
	}

	for _, child := range title.Children {
		if err := s.saveTitle(ctx, library, source, &item.ID, slug, child, tagged, found); err != nil {
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

	source, err := s.items.SaveSource(ctx, items.MediaSource{
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
