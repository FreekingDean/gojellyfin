package dto

import (
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func Describe(dto *api.BaseItemDto, described items.Description) {
	genres := make([]string, 0, len(described.Genres))
	for _, genre := range described.Genres {
		genres = append(genres, genre.Name)
	}

	people := make([]api.BaseItemPerson, 0, len(described.People))
	for _, person := range described.People {
		credited := api.BaseItemPerson{
			Id:   apiutil.Ptr(person.ID),
			Name: apiutil.Ptr(person.Name),
			Type: apiutil.Ptr(api.PersonKind(person.Kind)),
		}
		if person.Role != "" {
			credited.Role = apiutil.Ptr(person.Role)
		}
		if person.ImageTag != "" {
			credited.PrimaryImageTag = apiutil.Ptr(person.ImageTag)
		}
		people = append(people, credited)
	}

	dto.Genres = &genres
	dto.GenreItems = apiutil.Ptr(namePairs(described.Genres))
	dto.Studios = apiutil.Ptr(namePairs(described.Studios))
	dto.People = &people
}

func namePairs(named []items.Named) []api.NameGuidPair {
	pairs := make([]api.NameGuidPair, 0, len(named))
	for _, each := range named {
		pairs = append(pairs, api.NameGuidPair{Id: apiutil.Ptr(each.ID), Name: apiutil.Ptr(each.Name)})
	}

	return pairs
}
