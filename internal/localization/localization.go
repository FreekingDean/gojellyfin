package localization

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

//go:embed data/localization.json
var data []byte

const DefaultRatingCountry = "US"

type Culture struct {
	Name                string
	TwoLetterISOName    string
	ThreeLetterISONames []string
}

type Country struct {
	Name               string
	DisplayName        string
	TwoLetterISOName   string
	ThreeLetterISOName string
	ParentalRatings    []ParentalRating
}

type Option struct {
	Name  string
	Value string
}

type ParentalRating struct {
	Name  string
	Level *int
}

type document struct {
	Countries       map[string]Country
	Cultures        []Culture
	Options         []Option
	FallbackRatings []ParentalRating
}

type Service struct {
	countries       []Country
	byCode          map[string]Country
	cultures        []Culture
	options         []Option
	fallbackRatings []ParentalRating
}

func New() (*Service, error) {
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing localization data: %w", err)
	}
	if len(doc.Countries) == 0 || len(doc.Cultures) == 0 || len(doc.Options) == 0 {
		return nil, errors.New("localization data is missing countries, cultures or options")
	}
	if _, found := doc.Countries[DefaultRatingCountry]; !found {
		return nil, fmt.Errorf("localization data has no country %q", DefaultRatingCountry)
	}
	for _, culture := range doc.Cultures {
		if len(culture.ThreeLetterISONames) == 0 {
			return nil, fmt.Errorf("culture %q has no three letter name", culture.Name)
		}
	}
	if err := checkLevels("FallbackRatings", doc.FallbackRatings); err != nil {
		return nil, err
	}
	for code, country := range doc.Countries {
		if err := checkLevels(code, country.ParentalRatings); err != nil {
			return nil, err
		}
	}

	service := &Service{
		byCode:          make(map[string]Country, len(doc.Countries)),
		cultures:        doc.Cultures,
		options:         doc.Options,
		fallbackRatings: doc.FallbackRatings,
	}
	for code, country := range doc.Countries {
		country.Name = code
		country.TwoLetterISOName = code
		service.byCode[code] = country
		service.countries = append(service.countries, country)
	}
	slices.SortFunc(service.countries, func(a, b Country) int { return cmp.Compare(a.DisplayName, b.DisplayName) })

	return service, nil
}

func (s *Service) Cultures() []Culture {
	return slices.Clone(s.cultures)
}

func (s *Service) Countries() []Country {
	return slices.Clone(s.countries)
}

func (s *Service) Options() []Option {
	return slices.Clone(s.options)
}

func (s *Service) ParentalRatings() []ParentalRating {
	return s.ParentalRatingsFor(DefaultRatingCountry)
}

func (s *Service) ParentalRatingsFor(countryCode string) []ParentalRating {
	ratings := slices.Clone(s.byCode[strings.ToUpper(countryCode)].ParentalRatings)
	ratings = withFallbacks(ratings, []fallbackRating{{"Approved", 0}, {"10", 10}, {"13", 13}, {"14", 14}})
	if !slices.ContainsFunc(ratings, func(r ParentalRating) bool { return *r.Level >= 21 }) {
		ratings = append(ratings, rating("21", 21))
	}
	ratings = withFallbacks(ratings, []fallbackRating{{"XXX", 1000}, {"Banned", 1001}})

	slices.SortStableFunc(ratings, func(a, b ParentalRating) int { return cmp.Compare(*a.Level, *b.Level) })

	return append([]ParentalRating{{Name: "Unrated"}}, ratings...)
}

func (s *Service) RatingLevel(name string) (int, bool) {
	if level, found := levelIn(s.byCode[DefaultRatingCountry].ParentalRatings, name); found {
		return level, true
	}
	if level, found := levelIn(s.fallbackRatings, name); found {
		return level, true
	}
	for _, country := range s.countries {
		if level, found := levelIn(country.ParentalRatings, name); found {
			return level, true
		}
	}

	return 0, false
}

type fallbackRating struct {
	name  string
	level int
}

func withFallbacks(ratings []ParentalRating, fallbacks []fallbackRating) []ParentalRating {
	for _, fallback := range fallbacks {
		if !slices.ContainsFunc(ratings, func(r ParentalRating) bool { return *r.Level == fallback.level }) {
			ratings = append(ratings, rating(fallback.name, fallback.level))
		}
	}

	return ratings
}

func checkLevels(owner string, ratings []ParentalRating) error {
	for _, rating := range ratings {
		if rating.Level == nil {
			return fmt.Errorf("rating %q in %q has no level", rating.Name, owner)
		}
	}

	return nil
}

func levelIn(ratings []ParentalRating, name string) (int, bool) {
	index := slices.IndexFunc(ratings, func(r ParentalRating) bool { return strings.EqualFold(r.Name, name) })
	if index < 0 {
		return 0, false
	}

	return *ratings[index].Level, true
}

func rating(name string, level int) ParentalRating {
	return ParentalRating{Name: name, Level: &level}
}
