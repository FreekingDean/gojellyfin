package localization

import (
	"cmp"
	"slices"
	"testing"
)

func TestService_Cultures(t *testing.T) {
	cultures := testService(t).Cultures()
	if len(cultures) != 191 {
		t.Fatalf("got %d cultures, want 191", len(cultures))
	}

	for _, want := range []Culture{
		{Name: "English", TwoLetterISOName: "en", ThreeLetterISONames: []string{"eng"}},
		{Name: "German", TwoLetterISOName: "de", ThreeLetterISONames: []string{"ger", "deu"}},
		{Name: "Japanese", TwoLetterISOName: "ja", ThreeLetterISONames: []string{"jpn"}},
	} {
		index := slices.IndexFunc(cultures, func(c Culture) bool { return c.TwoLetterISOName == want.TwoLetterISOName })
		if index < 0 {
			t.Fatalf("%q missing", want.TwoLetterISOName)
		}
		if got := cultures[index]; got.Name != want.Name || !slices.Equal(got.ThreeLetterISONames, want.ThreeLetterISONames) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	}
}

func TestService_Countries(t *testing.T) {
	countries := testService(t).Countries()
	if len(countries) != 139 {
		t.Fatalf("got %d countries, want 139", len(countries))
	}

	t.Run("carry their ISO names and rating tables", func(t *testing.T) {
		for code, want := range map[string]Country{
			"US": {Name: "US", DisplayName: "United States", TwoLetterISOName: "US", ThreeLetterISOName: "USA"},
			"GB": {Name: "GB", DisplayName: "United Kingdom", TwoLetterISOName: "GB", ThreeLetterISOName: "GBR"},
			"JP": {Name: "JP", DisplayName: "Japan", TwoLetterISOName: "JP", ThreeLetterISOName: "JPN"},
			"AF": {Name: "AF", DisplayName: "Afghanistan", TwoLetterISOName: "AF", ThreeLetterISOName: "AFG"},
		} {
			index := slices.IndexFunc(countries, func(c Country) bool { return c.TwoLetterISOName == code })
			if index < 0 {
				t.Fatalf("%q missing", code)
			}

			got := countries[index]
			if got.Name != want.Name || got.DisplayName != want.DisplayName || got.ThreeLetterISOName != want.ThreeLetterISOName {
				t.Errorf("%q: got %+v, want %+v", code, got, want)
			}
		}

		ratingTables := 0
		for _, country := range countries {
			if len(country.ParentalRatings) > 0 {
				ratingTables++
			}
		}
		if ratingTables != 23 {
			t.Errorf("got %d countries with a rating table, want 23", ratingTables)
		}
	})

	t.Run("are sorted by display name", func(t *testing.T) {
		if !slices.IsSortedFunc(countries, func(a, b Country) int {
			return cmp.Compare(a.DisplayName, b.DisplayName)
		}) {
			t.Error("countries are not sorted by display name")
		}
	})
}

func TestService_Options(t *testing.T) {
	options := testService(t).Options()
	if len(options) != 71 {
		t.Fatalf("got %d options, want 71", len(options))
	}

	for value, name := range map[string]string{"en-US": "English", "fr": "Français", "ja": "日本語", "zh-TW": "漢語 (繁體字)"} {
		index := slices.IndexFunc(options, func(o Option) bool { return o.Value == value })
		if index < 0 {
			t.Fatalf("%q missing", value)
		}
		if got := options[index].Name; got != name {
			t.Errorf("%q: got %q, want %q", value, got, name)
		}
	}
}

func TestService_ParentalRatingsFor(t *testing.T) {
	for _, tc := range []struct {
		country string
		count   int
		levels  map[string]int
	}{
		{country: "US", count: 54, levels: map[string]int{"PG-13": 13, "TV-MA": 17, "21": 21, "XXX": 1000}},
		{country: "de", count: 24, levels: map[string]int{"FSK-16": 16, "FSK-18": 18, "13": 13, "Banned": 1001}},
		{country: "JP", count: 18, levels: map[string]int{"G": 0, "R15+": 15, "Z": 18, "21": 21}},
		{country: "AF", count: 8, levels: map[string]int{"Approved": 0, "14": 14, "Banned": 1001}},
		{country: "zz", count: 8, levels: map[string]int{"Approved": 0, "10": 10, "XXX": 1000}},
	} {
		t.Run(tc.country, func(t *testing.T) {
			ratings := testService(t).ParentalRatingsFor(tc.country)
			if len(ratings) != tc.count {
				t.Fatalf("got %d ratings, want %d", len(ratings), tc.count)
			}
			if ratings[0].Name != "Unrated" || ratings[0].Level != nil {
				t.Errorf("got first rating %q, want an unlevelled Unrated", ratings[0].Name)
			}
			if !slices.IsSortedFunc(ratings[1:], func(a, b ParentalRating) int { return *a.Level - *b.Level }) {
				t.Error("ratings are not sorted by level")
			}

			for name, want := range tc.levels {
				index := slices.IndexFunc(ratings, func(r ParentalRating) bool { return r.Name == name })
				if index < 0 {
					t.Fatalf("%q missing", name)
				}
				if got := *ratings[index].Level; got != want {
					t.Errorf("%q: got %d, want %d", name, got, want)
				}
			}
		})
	}
}

func TestService_ParentalRatings(t *testing.T) {
	service := testService(t)
	if len(service.ParentalRatings()) != len(service.ParentalRatingsFor("US")) {
		t.Error("the default ratings are not the United States ratings")
	}
}

func TestService_RatingLevel(t *testing.T) {
	service := testService(t)
	for name, want := range map[string]int{
		"PG-13":  13,
		"tv-ma":  17,
		"FSK-18": 18,
		"18A":    18,
		"XXX":    1000,
	} {
		got, found := service.RatingLevel(name)
		if !found {
			t.Errorf("%q: not found", name)
			continue
		}
		if got != want {
			t.Errorf("%q: got %d, want %d", name, got, want)
		}
	}

	if _, found := service.RatingLevel("Not A Rating"); found {
		t.Error("an unknown rating should not resolve")
	}
}

func testService(t *testing.T) *Service {
	t.Helper()

	service, err := New()
	if err != nil {
		t.Fatal(err)
	}

	return service
}
