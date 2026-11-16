package items

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	creditmodel "github.com/FreekingDean/gojellyfin/internal/store/credit"
	genremodel "github.com/FreekingDean/gojellyfin/internal/store/genre"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	studiomodel "github.com/FreekingDean/gojellyfin/internal/store/studio"
)

const (
	LockedGenres  = "Genres"
	LockedStudios = "Studios"
	LockedCast    = "Cast"
)

type Person struct {
	Named
	Kind CreditKind
	Role string
}

type Description struct {
	Genres  []Named
	Studios []Named
	People  []Person
}

func (s *Service) Description(ctx context.Context, id uuid.UUID) (Description, error) {
	genres, err := s.store.Genre.Query().
		Where(genremodel.HasItemsWith(itemmodel.ID(id))).
		Order(genremodel.ByName()).
		All(ctx)
	if err != nil {
		return Description{}, fmt.Errorf("failed to query the genres: %w", err)
	}

	studios, err := s.store.Studio.Query().
		Where(studiomodel.HasItemsWith(itemmodel.ID(id))).
		Order(studiomodel.ByName()).
		All(ctx)
	if err != nil {
		return Description{}, fmt.Errorf("failed to query the studios: %w", err)
	}

	credits, err := s.store.Credit.Query().
		Where(creditmodel.HasItemWith(itemmodel.ID(id))).
		WithPerson().
		Order(creditmodel.BySortOrder(), creditmodel.ByID()).
		All(ctx)
	if err != nil {
		return Description{}, fmt.Errorf("failed to query the credits: %w", err)
	}

	described := Description{
		Genres:  make([]Named, 0, len(genres)),
		Studios: make([]Named, 0, len(studios)),
		People:  make([]Person, 0, len(credits)),
	}
	for _, genre := range genres {
		described.Genres = append(described.Genres, Named{ID: genre.ID, Name: genre.Name})
	}
	for _, studio := range studios {
		described.Studios = append(described.Studios, Named{ID: studio.ID, Name: studio.Name})
	}
	for _, credit := range credits {
		person := credit.Edges.Person
		if person == nil {
			continue
		}
		described.People = append(described.People, Person{
			Named: Named{ID: person.ID, Name: person.Name, ImageTag: imageTag(person.ImageURL)},
			Kind:  credit.Kind,
			Role:  credit.Role,
		})
	}

	return described, nil
}

func (d Description) edited(metadata Metadata) []string {
	var changed []string
	if metadata.Genres != nil && !sameNames(describedNames(d.Genres), *metadata.Genres) {
		changed = append(changed, LockedGenres)
	}
	if metadata.Studios != nil && !sameNames(describedNames(d.Studios), *metadata.Studios) {
		changed = append(changed, LockedStudios)
	}
	if metadata.People != nil && !sameCredits(d.People, *metadata.People) {
		changed = append(changed, LockedCast)
	}

	return changed
}

func describedNames(named []Named) []string {
	listed := make([]string, 0, len(named))
	for _, each := range named {
		listed = append(listed, each.Name)
	}

	return listed
}

func sameNames(stored, sent []string) bool {
	stored = slices.Sorted(slices.Values(stored))
	sent = slices.Compact(slices.Sorted(slices.Values(sent)))

	return slices.Equal(stored, sent)
}

func sameCredits(stored []Person, sent []Credit) bool {
	return slices.EqualFunc(stored, sent, func(person Person, credit Credit) bool {
		return person.Name == credit.Name && person.Kind == credit.Kind && person.Role == credit.Role
	})
}
