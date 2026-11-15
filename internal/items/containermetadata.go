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

const CreditKindUnknown = creditmodel.KindUnknown

type Named struct {
	ID       uuid.UUID
	Name     string
	ImageTag string
}

type MetadataQuery struct {
	Viewer Viewer

	LibraryID  *uuid.UUID
	ItemID     *uuid.UUID
	Kinds      []Kind
	SearchTerm string
	Name       string
	StartIndex int
	Limit      int
}

func (q MetadataQuery) items() []predicate.Item {
	filters := append([]predicate.Item{itemmodel.DeletedAtIsNil()}, q.Viewer.visible()...)
	if q.LibraryID != nil {
		filters = append(filters, inLibrary(*q.LibraryID))
	}
	if q.ItemID != nil {
		filters = append(filters, itemmodel.ID(*q.ItemID))
	}
	if q.Kinds != nil {
		filters = append(filters, itemmodel.KindIn(q.Kinds...))
	}

	return filters
}

func (s *Service) DistinctGenres(ctx context.Context, query MetadataQuery) ([]Named, int, error) {
	genres := s.store.Genre.Query().Where(genremodel.HasItemsWith(query.items()...))
	if query.SearchTerm != "" {
		genres = genres.Where(genremodel.NameContainsFold(query.SearchTerm))
	}
	if query.Name != "" {
		genres = genres.Where(genremodel.NameEqualFold(query.Name))
	}

	return namedPage(ctx, genres.Order(genremodel.ByName()), query, func(record *store.Genre) Named {
		return Named{ID: record.ID, Name: record.Name}
	})
}

func (s *Service) DistinctStudios(ctx context.Context, query MetadataQuery) ([]Named, int, error) {
	studios := s.store.Studio.Query().Where(studiomodel.HasItemsWith(query.items()...))
	if query.SearchTerm != "" {
		studios = studios.Where(studiomodel.NameContainsFold(query.SearchTerm))
	}
	if query.Name != "" {
		studios = studios.Where(studiomodel.NameEqualFold(query.Name))
	}

	return namedPage(ctx, studios.Order(studiomodel.ByName()), query, func(record *store.Studio) Named {
		return Named{ID: record.ID, Name: record.Name}
	})
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
	if query.Name != "" {
		people = people.Where(personmodel.NameEqualFold(query.Name))
	}

	return namedPage(ctx, people.Order(personmodel.ByName()), query, func(record *store.Person) Named {
		return Named{ID: record.ID, Name: record.Name, ImageTag: imageTag(record.ImageURL)}
	})
}

func (s *Service) GenreNamed(ctx context.Context, query MetadataQuery, name string) (Named, error) {
	return only(s.DistinctGenres(ctx, query.named(name)))("genre", name)
}

func (s *Service) StudioNamed(ctx context.Context, query MetadataQuery, name string) (Named, error) {
	return only(s.DistinctStudios(ctx, query.named(name)))("studio", name)
}

func (s *Service) PersonNamed(ctx context.Context, query MetadataQuery, name string) (Named, error) {
	return only(s.DistinctPeople(ctx, query.named(name), nil))("person", name)
}

func (q MetadataQuery) named(name string) MetadataQuery {
	q.Name = name
	q.StartIndex = 0
	q.Limit = 1

	return q
}

func only(named []Named, _ int, err error) func(what, name string) (Named, error) {
	return func(what, name string) (Named, error) {
		if err != nil {
			return Named{}, err
		}
		if len(named) == 0 {
			return Named{}, fmt.Errorf("%w: no %s is named %q", ErrNotFound, what, name)
		}

		return named[0], nil
	}
}

type pageable[Q, T any] interface {
	Clone() Q
	Count(ctx context.Context) (int, error)
	Offset(offset int) Q
	Limit(limit int) Q
	All(ctx context.Context) ([]*T, error)
}

func namedPage[Q pageable[Q, T], T any](ctx context.Context, rows Q, query MetadataQuery, named func(*T) Named) ([]Named, int, error) {
	total, err := rows.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count names: %w", err)
	}

	if query.StartIndex > 0 {
		rows = rows.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		rows = rows.Limit(query.Limit)
	}

	records, err := rows.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query names: %w", err)
	}

	found := make([]Named, 0, len(records))
	for _, record := range records {
		found = append(found, named(record))
	}

	return found, total, nil
}

func (s *Service) PersonImage(ctx context.Context, id uuid.UUID) (string, error) {
	return s.personImage(ctx, personmodel.ID(id), id.String())
}

func (s *Service) PersonImageNamed(ctx context.Context, name string) (string, error) {
	return s.personImage(ctx, personmodel.NameEqualFold(name), name)
}

func (s *Service) personImage(ctx context.Context, which predicate.Person, described string) (string, error) {
	person, err := s.store.Person.Query().
		Where(which, personmodel.ImageURLNEQ("")).
		First(ctx)
	if store.IsNotFound(err) {
		return "", fmt.Errorf("%w: no picture of %q", ErrNotFound, described)
	}
	if err != nil {
		return "", fmt.Errorf("failed to find %q: %w", described, err)
	}

	return person.ImageURL, nil
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

func (s *Service) DistinctOfficialRatings(ctx context.Context, query MetadataQuery) ([]string, error) {
	ratings, err := s.query(query.Viewer).
		Where(query.items()...).
		Where(itemmodel.OfficialRatingNotNil(), itemmodel.OfficialRatingNEQ("")).
		Order(itemmodel.ByOfficialRating()).
		Unique(true).
		Select(itemmodel.FieldOfficialRating).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query official ratings: %w", err)
	}

	return ratings, nil
}

func replaceCredits(ctx context.Context, tx *store.Tx, itemID uuid.UUID, people []Credit) error {
	if _, err := tx.Credit.Delete().
		Where(creditmodel.HasItemWith(itemmodel.ID(itemID))).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear the credits: %w", err)
	}

	for _, person := range people {
		create := tx.Person.Create().SetName(person.Name)
		if person.ImageURL != "" {
			create = create.SetImageURL(person.ImageURL)
		}
		id, err := create.
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
	return nameIDs(names, func(name string) (uuid.UUID, error) {
		return tx.Genre.Create().SetName(name).OnConflictColumns(genremodel.FieldName).UpdateName().ID(ctx)
	})
}

func studioIDs(ctx context.Context, tx *store.Tx, names []string) ([]uuid.UUID, error) {
	return nameIDs(names, func(name string) (uuid.UUID, error) {
		return tx.Studio.Create().SetName(name).OnConflictColumns(studiomodel.FieldName).UpdateName().ID(ctx)
	})
}

func nameIDs(names []string, save func(name string) (uuid.UUID, error)) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		id, err := save(name)
		if err != nil {
			return nil, fmt.Errorf("failed to save %q: %w", name, err)
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}

	return ids, nil
}
