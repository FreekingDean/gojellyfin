package localization

import (
	"context"
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestGetCultures(t *testing.T) {
	response, err := testServer(t).GetCultures(context.Background(), api.GetCulturesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	dtos := response.(api.GetCultures200JSONResponse)
	if len(dtos) != 191 {
		t.Fatalf("got %d cultures, want 191", len(dtos))
	}

	for _, want := range []api.CultureDto{
		{
			Name:                        apiutil.Ptr("English"),
			DisplayName:                 apiutil.Ptr("English"),
			TwoLetterISOLanguageName:    apiutil.Ptr("en"),
			ThreeLetterISOLanguageName:  apiutil.Ptr("eng"),
			ThreeLetterISOLanguageNames: &[]string{"eng"},
		},
		{
			Name:                        apiutil.Ptr("German"),
			DisplayName:                 apiutil.Ptr("German"),
			TwoLetterISOLanguageName:    apiutil.Ptr("de"),
			ThreeLetterISOLanguageName:  apiutil.Ptr("ger"),
			ThreeLetterISOLanguageNames: &[]string{"ger", "deu"},
		},
	} {
		index := slices.IndexFunc(dtos, func(c api.CultureDto) bool {
			return apiutil.Deref(c.TwoLetterISOLanguageName) == apiutil.Deref(want.TwoLetterISOLanguageName)
		})
		if index < 0 {
			t.Fatalf("%q missing", apiutil.Deref(want.TwoLetterISOLanguageName))
		}

		got := dtos[index]
		if apiutil.Deref(got.Name) != apiutil.Deref(want.Name) || apiutil.Deref(got.DisplayName) != apiutil.Deref(want.DisplayName) {
			t.Errorf("got name %q, want %q", apiutil.Deref(got.Name), apiutil.Deref(want.Name))
		}
		if apiutil.Deref(got.ThreeLetterISOLanguageName) != apiutil.Deref(want.ThreeLetterISOLanguageName) {
			t.Errorf("got three letter name %q, want %q", apiutil.Deref(got.ThreeLetterISOLanguageName), apiutil.Deref(want.ThreeLetterISOLanguageName))
		}
		if !slices.Equal(apiutil.Deref(got.ThreeLetterISOLanguageNames), apiutil.Deref(want.ThreeLetterISOLanguageNames)) {
			t.Errorf("got three letter names %v, want %v", apiutil.Deref(got.ThreeLetterISOLanguageNames), apiutil.Deref(want.ThreeLetterISOLanguageNames))
		}
	}
}

func TestGetCountries(t *testing.T) {
	response, err := testServer(t).GetCountries(context.Background(), api.GetCountriesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	dtos := response.(api.GetCountries200JSONResponse)
	if len(dtos) != 139 {
		t.Fatalf("got %d countries, want 139", len(dtos))
	}

	for two, three := range map[string]string{"US": "USA", "GB": "GBR", "JP": "JPN", "BR": "BRA"} {
		index := slices.IndexFunc(dtos, func(c api.CountryInfo) bool {
			return apiutil.Deref(c.TwoLetterISORegionName) == two
		})
		if index < 0 {
			t.Fatalf("%q missing", two)
		}
		if got := apiutil.Deref(dtos[index].ThreeLetterISORegionName); got != three {
			t.Errorf("%q: got %q, want %q", two, got, three)
		}
		if apiutil.Deref(dtos[index].Name) != two || apiutil.Deref(dtos[index].DisplayName) == "" {
			t.Errorf("%q: got name %q and display name %q", two, apiutil.Deref(dtos[index].Name), apiutil.Deref(dtos[index].DisplayName))
		}
	}
}

func TestGetLocalizationOptions(t *testing.T) {
	response, err := testServer(t).GetLocalizationOptions(context.Background(), api.GetLocalizationOptionsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	dtos := response.(api.GetLocalizationOptions200JSONResponse)
	if len(dtos) != 71 {
		t.Fatalf("got %d options, want 71", len(dtos))
	}

	for value, name := range map[string]string{"en-US": "English", "fr": "Français", "ja": "日本語", "zh-TW": "漢語 (繁體字)"} {
		index := slices.IndexFunc(dtos, func(o api.LocalizationOption) bool {
			return apiutil.Deref(o.Value) == value
		})
		if index < 0 {
			t.Fatalf("%q missing", value)
		}
		if got := apiutil.Deref(dtos[index].Name); got != name {
			t.Errorf("%q: got %q, want %q", value, got, name)
		}
	}
}

func TestGetParentalRatings(t *testing.T) {
	response, err := testServer(t).GetParentalRatings(context.Background(), api.GetParentalRatingsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	dtos := response.(api.GetParentalRatings200JSONResponse)
	if len(dtos) != 54 {
		t.Fatalf("got %d ratings, want 54", len(dtos))
	}
	if got := apiutil.Deref(dtos[0].Name); got != "Unrated" || dtos[0].Value != nil {
		t.Errorf("got first rating %q with value %v, want an unvalued Unrated", got, dtos[0].Value)
	}

	for name, want := range map[string]int32{
		"G": 0, "TV-Y7": 7, "PG": 10, "PG-13": 13, "TV-14": 14, "NC-17": 17,
		"21": 21, "XXX": 1000, "Banned": 1001,
	} {
		index := slices.IndexFunc(dtos, func(r api.ParentalRating) bool { return apiutil.Deref(r.Name) == name })
		if index < 0 {
			t.Fatalf("%q missing", name)
		}
		if got := apiutil.Deref(dtos[index].Value); got != want {
			t.Errorf("%q: got %d, want %d", name, got, want)
		}
	}

	if !slices.IsSortedFunc(dtos[1:], func(a, b api.ParentalRating) int {
		return int(apiutil.Deref(a.Value) - apiutil.Deref(b.Value))
	}) {
		t.Error("ratings are not sorted by value")
	}
}

func testServer(t *testing.T) *Server {
	t.Helper()

	service, err := localization.New()
	if err != nil {
		t.Fatal(err)
	}

	return New(service)
}
