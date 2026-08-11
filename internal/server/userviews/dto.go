package userviews

import (
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func groupingOption(library *libraries.Library) api.SpecialViewOptionDto {
	return api.SpecialViewOptionDto{
		Id:   apiutil.Ptr(library.ID.String()),
		Name: apiutil.Ptr(library.Name),
	}
}
