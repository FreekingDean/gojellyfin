package localization

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	localization *localization.Service
}

func New(localization *localization.Service) *Server {
	return &Server{localization: localization}
}

func (s *Server) GetCultures(ctx context.Context, request api.GetCulturesRequestObject) (api.GetCulturesResponseObject, error) {
	return api.GetCultures200JSONResponse(dto.CultureDtos(s.localization.Cultures())), nil
}

func (s *Server) GetCountries(ctx context.Context, request api.GetCountriesRequestObject) (api.GetCountriesResponseObject, error) {
	return api.GetCountries200JSONResponse(dto.CountryInfos(s.localization.Countries())), nil
}

func (s *Server) GetLocalizationOptions(ctx context.Context, request api.GetLocalizationOptionsRequestObject) (api.GetLocalizationOptionsResponseObject, error) {
	return api.GetLocalizationOptions200JSONResponse(localizationOptions(s.localization.Options())), nil
}

func (s *Server) GetParentalRatings(ctx context.Context, request api.GetParentalRatingsRequestObject) (api.GetParentalRatingsResponseObject, error) {
	return api.GetParentalRatings200JSONResponse(dto.ParentalRatings(s.localization.ParentalRatings())), nil
}
