package localization

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
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
		{Name: apiutil.Ptr("English"), Value: apiutil.Ptr("en-US")},
	}), nil
}

func (s *Server) GetParentalRatings(ctx context.Context, request api.GetParentalRatingsRequestObject) (api.GetParentalRatingsResponseObject, error) {
	return api.GetParentalRatings200JSONResponse([]api.ParentalRating{
		{Name: apiutil.Ptr("G"), Value: apiutil.Ptr(int32(1))},
		{Name: apiutil.Ptr("PG"), Value: apiutil.Ptr(int32(5))},
		{Name: apiutil.Ptr("PG-13"), Value: apiutil.Ptr(int32(7))},
		{Name: apiutil.Ptr("R"), Value: apiutil.Ptr(int32(9))},
		{Name: apiutil.Ptr("NC-17"), Value: apiutil.Ptr(int32(10))},
	}), nil
}

func culture(displayName, twoLetter, threeLetter string) api.CultureDto {
	return api.CultureDto{
		Name:                        apiutil.Ptr(displayName),
		DisplayName:                 apiutil.Ptr(displayName),
		TwoLetterISOLanguageName:    apiutil.Ptr(twoLetter),
		ThreeLetterISOLanguageName:  apiutil.Ptr(threeLetter),
		ThreeLetterISOLanguageNames: &[]string{threeLetter},
	}
}

func country(displayName, twoLetter, threeLetter string) api.CountryInfo {
	return api.CountryInfo{
		Name:                     apiutil.Ptr(displayName),
		DisplayName:              apiutil.Ptr(displayName),
		TwoLetterISORegionName:   apiutil.Ptr(twoLetter),
		ThreeLetterISORegionName: apiutil.Ptr(threeLetter),
	}
}
