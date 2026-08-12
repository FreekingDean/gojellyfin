package dto

import (
	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func CultureDtos(cultures []localization.Culture) []api.CultureDto {
	dtos := make([]api.CultureDto, 0, len(cultures))
	for _, culture := range cultures {
		dtos = append(dtos, api.CultureDto{
			Name:                        apiutil.Ptr(culture.Name),
			DisplayName:                 apiutil.Ptr(culture.Name),
			TwoLetterISOLanguageName:    apiutil.Ptr(culture.TwoLetterISOName),
			ThreeLetterISOLanguageName:  apiutil.Ptr(culture.ThreeLetterISONames[0]),
			ThreeLetterISOLanguageNames: apiutil.Ptr(culture.ThreeLetterISONames),
		})
	}

	return dtos
}

func CountryInfos(countries []localization.Country) []api.CountryInfo {
	dtos := make([]api.CountryInfo, 0, len(countries))
	for _, country := range countries {
		dtos = append(dtos, api.CountryInfo{
			Name:                     apiutil.Ptr(country.Name),
			DisplayName:              apiutil.Ptr(country.DisplayName),
			TwoLetterISORegionName:   apiutil.Ptr(country.TwoLetterISOName),
			ThreeLetterISORegionName: apiutil.Ptr(country.ThreeLetterISOName),
		})
	}

	return dtos
}

func ParentalRatings(ratings []localization.ParentalRating) []api.ParentalRating {
	dtos := make([]api.ParentalRating, 0, len(ratings))
	for _, rating := range ratings {
		dto := api.ParentalRating{Name: apiutil.Ptr(rating.Name)}
		if rating.Level != nil {
			dto.Value = apiutil.Ptr(int32(*rating.Level))
		}
		dtos = append(dtos, dto)
	}

	return dtos
}
