package items

import (
	"context"
	"fmt"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (s *Service) SweepJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.SweepItems,
		Category:    "Library",
		Description: "Removes titles no source reports a file for any more, and the genres, studios and people nothing credits.",
		Startable:   true,
		Run:         s.sweep,
	}
}

func (s *Service) sweep(ctx context.Context) error {
	everything, err := s.store.Item.Query().Where(itemmodel.DeletedAtIsNil()).IDs(ctx)
	if err != nil {
		return err
	}

	if err := s.SweepUnreachable(ctx, everything); err != nil {
		return err
	}

	return s.SweepNames(ctx)
}

var unreferencedNames = []string{
	`DELETE FROM genres WHERE id IN (
		SELECT id FROM genres AS g
		WHERE NOT EXISTS (SELECT 1 FROM item_genres AS e WHERE e.genre_id = g.id)
		FOR UPDATE SKIP LOCKED)`,
	`DELETE FROM studios WHERE id IN (
		SELECT id FROM studios AS s
		WHERE NOT EXISTS (SELECT 1 FROM item_studios AS e WHERE e.studio_id = s.id)
		FOR UPDATE SKIP LOCKED)`,
	`DELETE FROM persons WHERE id IN (
		SELECT id FROM persons AS p
		WHERE NOT EXISTS (SELECT 1 FROM credits AS c WHERE c.person_credits = p.id)
		FOR UPDATE SKIP LOCKED)`,
}

func (s *Service) SweepNames(ctx context.Context) error {
	for _, statement := range unreferencedNames {
		if _, err := s.store.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("failed to sweep unreferenced names: %w", err)
		}
	}

	return nil
}
