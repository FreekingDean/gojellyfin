package localization

import (
	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func localizationOptions(options []localization.Option) []api.LocalizationOption {
	dtos := make([]api.LocalizationOption, 0, len(options))
	for _, option := range options {
		dtos = append(dtos, api.LocalizationOption{
			Name:  apiutil.Ptr(option.Name),
			Value: apiutil.Ptr(option.Value),
		})
	}

	return dtos
}
