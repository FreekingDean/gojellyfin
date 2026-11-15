package health

import (
	"github.com/FreekingDean/gojellyfin/internal/health"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func healthDto(report health.Report) api.Health {
	checks := make([]api.HealthCheck, len(report.Checks))
	for i, check := range report.Checks {
		checks[i] = api.HealthCheck{
			Name:   check.Name,
			Status: api.HealthStatus(check.Status),
			Detail: check.Detail,
			At:     check.At,
		}
	}

	library := report.Library

	return api.Health{
		Checks: checks,
		Library: api.HealthLibrary{
			Movies:       int32(library.Movies),
			Shows:        int32(library.Shows),
			Episodes:     int32(library.Episodes),
			Unprobed:     int32(library.Unprobed),
			Unidentified: int32(library.Unidentified),
			RefreshedAt:  library.RefreshedAt,
		},
	}
}
