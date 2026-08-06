package localization

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dtos"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

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
		{Name: dtos.Ptr("English"), Value: dtos.Ptr("en-US")},
	}), nil
}

func (s *Server) GetParentalRatings(ctx context.Context, request api.GetParentalRatingsRequestObject) (api.GetParentalRatingsResponseObject, error) {
	return api.GetParentalRatings200JSONResponse([]api.ParentalRating{
		{Name: dtos.Ptr("G"), Value: dtos.Ptr(int32(1))},
		{Name: dtos.Ptr("PG"), Value: dtos.Ptr(int32(5))},
		{Name: dtos.Ptr("PG-13"), Value: dtos.Ptr(int32(7))},
		{Name: dtos.Ptr("R"), Value: dtos.Ptr(int32(9))},
		{Name: dtos.Ptr("NC-17"), Value: dtos.Ptr(int32(10))},
	}), nil
}

func culture(displayName, twoLetter, threeLetter string) api.CultureDto {
	return api.CultureDto{
		Name:                        dtos.Ptr(displayName),
		DisplayName:                 dtos.Ptr(displayName),
		TwoLetterISOLanguageName:    dtos.Ptr(twoLetter),
		ThreeLetterISOLanguageName:  dtos.Ptr(threeLetter),
		ThreeLetterISOLanguageNames: &[]string{threeLetter},
	}
}

func country(displayName, twoLetter, threeLetter string) api.CountryInfo {
	return api.CountryInfo{
		Name:                     dtos.Ptr(displayName),
		DisplayName:              dtos.Ptr(displayName),
		TwoLetterISORegionName:   dtos.Ptr(twoLetter),
		ThreeLetterISORegionName: dtos.Ptr(threeLetter),
	}
}
