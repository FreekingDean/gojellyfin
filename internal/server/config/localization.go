package config

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GetCultures(ctx context.Context, request api.GetCulturesRequestObject) (api.GetCulturesResponseObject, error) {
	return api.GetCultures200JSONResponse([]api.CultureDto{
		culture("English", "en", "eng"),
		culture("French", "fr", "fra"),
		culture("German", "de", "deu"),
		culture("Spanish", "es", "spa"),
		culture("Japanese", "ja", "jpn"),
	}), nil
}

func (s *Server) GetCountries(ctx context.Context, request api.GetCountriesRequestObject) (api.GetCountriesResponseObject, error) {
	return api.GetCountries200JSONResponse([]api.CountryInfo{
		country("United States", "US", "USA"),
		country("United Kingdom", "GB", "GBR"),
		country("Canada", "CA", "CAN"),
		country("Australia", "AU", "AUS"),
		country("Germany", "DE", "DEU"),
	}), nil
}

func (s *Server) GetLocalizationOptions(ctx context.Context, request api.GetLocalizationOptionsRequestObject) (api.GetLocalizationOptionsResponseObject, error) {
	return api.GetLocalizationOptions200JSONResponse([]api.LocalizationOption{
		{Name: ptr("English"), Value: ptr("en-US")},
	}), nil
}

func (s *Server) GetParentalRatings(ctx context.Context, request api.GetParentalRatingsRequestObject) (api.GetParentalRatingsResponseObject, error) {
	return api.GetParentalRatings200JSONResponse([]api.ParentalRating{
		{Name: ptr("G"), Value: ptr(int32(1))},
		{Name: ptr("PG"), Value: ptr(int32(5))},
		{Name: ptr("PG-13"), Value: ptr(int32(7))},
		{Name: ptr("R"), Value: ptr(int32(9))},
		{Name: ptr("NC-17"), Value: ptr(int32(10))},
	}), nil
}

func culture(displayName, twoLetter, threeLetter string) api.CultureDto {
	return api.CultureDto{
		Name:                        ptr(displayName),
		DisplayName:                 ptr(displayName),
		TwoLetterISOLanguageName:    ptr(twoLetter),
		ThreeLetterISOLanguageName:  ptr(threeLetter),
		ThreeLetterISOLanguageNames: &[]string{threeLetter},
	}
}

func country(displayName, twoLetter, threeLetter string) api.CountryInfo {
	return api.CountryInfo{
		Name:                     ptr(displayName),
		DisplayName:              ptr(displayName),
		TwoLetterISORegionName:   ptr(twoLetter),
		ThreeLetterISORegionName: ptr(threeLetter),
	}
}
