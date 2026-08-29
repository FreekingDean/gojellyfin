package items

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	creditmodal "github.com/FreekingDean/gojellyfin/internal/store/credit"
	genremodal "github.com/FreekingDean/gojellyfin/internal/store/genre"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	personmodal "github.com/FreekingDean/gojellyfin/internal/store/person"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	studiomodal "github.com/FreekingDean/gojellyfin/internal/store/studio"
)

type CreditKind = creditmodal.Kind

var ValidCreditKind = creditmodal.KindValidator

type ContainerMetadata struct {
	Genres  []string
	Studios []string
	Tags    []string
	People  []Person
}

type Person struct {
	Name string
	Kind CreditKind
}

type Named struct {
	ID   uuid.UUID
	Name string
}

type MetadataQuery struct {
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
		filters = append(filters, itemmodal.LibraryID(*q.LibraryID))
	}
	if q.ItemID != nil {
		filters = append(filters, itemmodal.ID(*q.ItemID))
	}
	if len(q.Kinds) > 0 {
		filters = append(filters, itemmodal.KindIn(q.Kinds...))
	}

	return filters
}

func (s *Service) DistinctGenres(ctx context.Context, query MetadataQuery) ([]Named, int, error) {
	genres := s.store.Genre.Query().Where(genremodal.HasItemsWith(query.items()...))
	if query.SearchTerm != "" {
		genres = genres.Where(genremodal.NameContainsFold(query.SearchTerm))
	}

	total, err := genres.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count genres: %w", err)
	}

	genres = genres.Order(genremodal.ByName())
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

func (s *Service) GenreByName(ctx context.Context, name string) (Named, error) {
	record, err := s.store.Genre.Query().Where(genremodal.NameEqualFold(name)).First(ctx)
	if err != nil {
		return Named{}, fmt.Errorf("failed to query genre by name: %w", err)
	}

	return Named{ID: record.ID, Name: record.Name}, nil
}

func (s *Service) DistinctStudios(ctx context.Context, query MetadataQuery) ([]Named, int, error) {
	studios := s.store.Studio.Query().Where(studiomodal.HasItemsWith(query.items()...))
	if query.SearchTerm != "" {
		studios = studios.Where(studiomodal.NameContainsFold(query.SearchTerm))
	}

	total, err := studios.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count studios: %w", err)
	}

	studios = studios.Order(studiomodal.ByName())
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
	credits := []predicate.Credit{creditmodal.HasItemWith(query.items()...)}
	if len(kinds) > 0 {
		credits = append(credits, creditmodal.KindIn(kinds...))
	}

	people := s.store.Person.Query().Where(personmodal.HasCreditsWith(credits...))
	if query.SearchTerm != "" {
		people = people.Where(personmodal.NameContainsFold(query.SearchTerm))
	}

	total, err := people.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count people: %w", err)
	}

	people = people.Order(personmodal.ByName())
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
	records, err := s.query().
		Where(query.items()...).
		Where(itemmodal.TagsNotNil()).
		Select(itemmodal.FieldTags).
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

func saveCredits(ctx context.Context, tx *store.Tx, itemID uuid.UUID, people []Person) error {
	if _, err := tx.Credit.Delete().Where(creditmodal.HasItemWith(itemmodal.ID(itemID))).Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear credits: %w", err)
	}
	if len(people) == 0 {
		return nil
	}

	builders := make([]*store.CreditCreate, 0, len(people))
	for _, person := range people {
		id, err := tx.Person.Create().
			SetName(person.Name).
			OnConflictColumns(personmodal.FieldName).
			UpdateName().
			ID(ctx)
		if err != nil {
			return fmt.Errorf("failed to save person: %w", err)
		}
		builders = append(builders, tx.Credit.Create().SetItemID(itemID).SetPersonID(id).SetKind(person.Kind))
	}

	if err := tx.Credit.CreateBulk(builders...).Exec(ctx); err != nil {
		return fmt.Errorf("failed to save credits: %w", err)
	}

	return nil
}

func genreIDs(ctx context.Context, tx *store.Tx, names []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		id, err := tx.Genre.Create().
			SetName(name).
			OnConflictColumns(genremodal.FieldName).
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
			OnConflictColumns(studiomodal.FieldName).
			UpdateName().
			ID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to save studio: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}
