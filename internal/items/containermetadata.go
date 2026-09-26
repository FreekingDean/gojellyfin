package items

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	creditmodel "github.com/FreekingDean/gojellyfin/internal/store/credit"
	genremodel "github.com/FreekingDean/gojellyfin/internal/store/genre"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	personmodel "github.com/FreekingDean/gojellyfin/internal/store/person"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	studiomodel "github.com/FreekingDean/gojellyfin/internal/store/studio"
)

type CreditKind = creditmodel.Kind

var ValidCreditKind = creditmodel.KindValidator

type Named struct {
	ID   uuid.UUID
	Name string
}

type MetadataQuery struct {
	Viewer Viewer

	LibraryID  *uuid.UUID
	ItemID     *uuid.UUID
	Kinds      []Kind
	SearchTerm string
	StartIndex int
	Limit      int
}

func (q MetadataQuery) items() []predicate.Item {
	filters := make([]predicate.Item, 0, 3)
	if q.LibraryID != nil {
		filters = append(filters, inLibrary(*q.LibraryID))
	}
	if q.ItemID != nil {
		filters = append(filters, itemmodel.ID(*q.ItemID))
	}
	if len(q.Kinds) > 0 {
		filters = append(filters, itemmodel.KindIn(q.Kinds...))
	}

	return filters
}

func (s *Service) DistinctGenres(ctx context.Context, query MetadataQuery) ([]Named, int, error) {
	genres := s.store.Genre.Query().Where(genremodel.HasItemsWith(query.items()...))
	if query.SearchTerm != "" {
		genres = genres.Where(genremodel.NameContainsFold(query.SearchTerm))
	}

	total, err := genres.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count genres: %w", err)
	}

	genres = genres.Order(genremodel.ByName())
	if query.StartIndex > 0 {
		genres = genres.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		genres = genres.Limit(query.Limit)
	}

	records, err := genres.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query genres: %w", err)
	}

	named := make([]Named, 0, len(records))
	for _, record := range records {
		named = append(named, Named{ID: record.ID, Name: record.Name})
	}

	return named, total, nil
}

func (s *Service) DistinctStudios(ctx context.Context, query MetadataQuery) ([]Named, int, error) {
	studios := s.store.Studio.Query().Where(studiomodel.HasItemsWith(query.items()...))
	if query.SearchTerm != "" {
		studios = studios.Where(studiomodel.NameContainsFold(query.SearchTerm))
	}

	total, err := studios.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count studios: %w", err)
	}

	studios = studios.Order(studiomodel.ByName())
	if query.StartIndex > 0 {
		studios = studios.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		studios = studios.Limit(query.Limit)
	}

	records, err := studios.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query studios: %w", err)
	}

	named := make([]Named, 0, len(records))
	for _, record := range records {
		named = append(named, Named{ID: record.ID, Name: record.Name})
	}

	return named, total, nil
}

func (s *Service) DistinctPeople(ctx context.Context, query MetadataQuery, kinds []CreditKind) ([]Named, int, error) {
	credits := []predicate.Credit{creditmodel.HasItemWith(query.items()...)}
	if len(kinds) > 0 {
		credits = append(credits, creditmodel.KindIn(kinds...))
	}

	people := s.store.Person.Query().Where(personmodel.HasCreditsWith(credits...))
	if query.SearchTerm != "" {
		people = people.Where(personmodel.NameContainsFold(query.SearchTerm))
	}

	total, err := people.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count people: %w", err)
	}

	people = people.Order(personmodel.ByName())
	if query.StartIndex > 0 {
		people = people.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		people = people.Limit(query.Limit)
	}

	records, err := people.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query people: %w", err)
	}

	named := make([]Named, 0, len(records))
	for _, record := range records {
		named = append(named, Named{ID: record.ID, Name: record.Name})
	}

	return named, total, nil
}

func (s *Service) DistinctTags(ctx context.Context, query MetadataQuery) ([]string, error) {
	records, err := s.query(query.Viewer).
		Where(query.items()...).
		Where(itemmodel.TagsNotNil()).
		Select(itemmodel.FieldTags).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}

	seen := make(map[string]bool)
	tags := make([]string, 0, len(records))
	for _, record := range records {
		for _, tag := range record.Tags {
			if seen[tag] {
				continue
			}
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	slices.Sort(tags)

	return tags, nil
}

func replaceCredits(ctx context.Context, tx *store.Tx, itemID uuid.UUID, people []Credit) error {
	if _, err := tx.Credit.Delete().
		Where(creditmodel.HasItemWith(itemmodel.ID(itemID))).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear the credits: %w", err)
	}

	for _, person := range people {
		id, err := tx.Person.Create().
			SetName(person.Name).
			OnConflictColumns(personmodel.FieldName).
			UpdateNewValues().
			ID(ctx)
		if err != nil {
			return fmt.Errorf("failed to save %q: %w", person.Name, err)
		}

		err = tx.Credit.Create().
			SetItemID(itemID).
			SetPersonID(id).
			SetKind(person.Kind).
			SetRole(person.Role).
			SetSortOrder(person.Order).
			OnConflictColumns(
				creditmodel.FieldKind,
				creditmodel.FieldRole,
				creditmodel.ItemColumn,
				creditmodel.PersonColumn,
			).
			UpdateNewValues().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to credit %q: %w", person.Name, err)
		}
	}

	return nil
}

func genreIDs(ctx context.Context, tx *store.Tx, names []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		id, err := tx.Genre.Create().
			SetName(name).
			OnConflictColumns(genremodel.FieldName).
			UpdateName().
			ID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to save genre: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

func studioIDs(ctx context.Context, tx *store.Tx, names []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		id, err := tx.Studio.Create().
			SetName(name).
			OnConflictColumns(studiomodel.FieldName).
			UpdateName().
			ID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to save studio: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}
