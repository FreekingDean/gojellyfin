package tmdb

import (
	"strconv"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/consts"
	"github.com/FreekingDean/gojellyfin/internal/items"
)

// Jellyfin's own key names, so an id written here means the same thing to a
// client as one written by upstream.
const (
	providerTmdb = "Tmdb"
	providerImdb = "Imdb"
)

const ratingCountry = "US"

func movieMetadata(movie *Movie) items.Metadata {
	premiere := date(movie.ReleaseDate)

	return items.Metadata{
		Name:                text(movie.Title),
		OriginalTitle:       text(movie.OriginalTitle),
		Overview:            text(movie.Overview),
		Status:              text(movie.Status),
		OfficialRating:      rating(movieCertification(movie)),
		CommunityRating:     score(movie.VoteAverage),
		PremiereDate:        premiere,
		ProductionYear:      year(premiere),
		Taglines:            list(movie.Tagline),
		ProductionLocations: countries(movie.ProductionCountries),
		ProviderIds:         providerIDs(movie.ID, movie.IMDbID),
	}
}

func seriesMetadata(series *Series) items.Metadata {
	premiere := date(series.FirstAirDate)

	metadata := items.Metadata{
		Name:                text(series.Name),
		OriginalTitle:       text(series.OriginalName),
		Overview:            text(series.Overview),
		Status:              text(series.Status),
		OfficialRating:      rating(seriesCertification(series)),
		CommunityRating:     score(series.VoteAverage),
		PremiereDate:        premiere,
		ProductionYear:      year(premiere),
		Taglines:            list(series.Tagline),
		ProductionLocations: countries(series.ProductionCountries),
		ProviderIds:         providerIDs(series.ID, series.ExternalIDs.IMDbID),
	}

	if !series.InProduction {
		metadata.EndDate = date(series.LastAirDate)
	}

	return metadata
}

func episodeMetadata(episode *Episode) items.Metadata {
	premiere := date(episode.AirDate)

	return items.Metadata{
		Name:            text(episode.Name),
		Overview:        text(episode.Overview),
		CommunityRating: score(episode.VoteAverage),
		PremiereDate:    premiere,
		ProductionYear:  year(premiere),
		ProviderIds:     providerIDs(episode.ID, episode.ExternalIDs.IMDbID),
	}
}

func movieCertification(movie *Movie) string {
	for _, released := range movie.ReleaseDates.Results {
		if released.Country != ratingCountry {
			continue
		}
		for _, release := range released.ReleaseDates {
			if release.Certification != "" {
				return release.Certification
			}
		}
	}

	return ""
}

func seriesCertification(series *Series) string {
	for _, rated := range series.ContentRatings.Results {
		if rated.Country == ratingCountry && rated.Rating != "" {
			return rated.Rating
		}
	}

	return ""
}

func providerIDs(tmdbID int, imdbID string) *map[string]string {
	ids := map[string]string{providerTmdb: strconv.Itoa(tmdbID)}
	if imdbID != "" {
		ids[providerImdb] = imdbID
	}

	return &ids
}

func countries(named []country) *[]string {
	if len(named) == 0 {
		return nil
	}

	names := make([]string, 0, len(named))
	for _, one := range named {
		names = append(names, one.Name)
	}

	return &names
}

func date(value string) *time.Time {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil
	}

	return &parsed
}

func year(premiere *time.Time) *int32 {
	if premiere == nil {
		return nil
	}
	value := int32(premiere.Year())

	return &value
}

func text(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func list(value string) *[]string {
	if value == "" {
		return nil
	}

	return &[]string{value}
}

func rating(value string) *consts.Rating {
	if value == "" {
		return nil
	}
	official := consts.Rating(value)

	return &official
}

func score(value float64) *float64 {
	if value == 0 {
		return nil
	}

	return &value
}
