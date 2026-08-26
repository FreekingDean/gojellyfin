package tmdb

import (
	"strconv"
	"time"

	gotmdb "github.com/cyruzin/golang-tmdb"

	"github.com/FreekingDean/gojellyfin/internal/consts"
	"github.com/FreekingDean/gojellyfin/internal/items"
)

const (
	providerTmdb = "Tmdb"
	providerImdb = "Imdb"
)

const ratingCountry = "US"

var statuses = map[string]string{
	"Returning Series": "Continuing",
	"Ended":            "Ended",
	"Canceled":         "Ended",
	"Planned":          "Unreleased",
	"In Production":    "Unreleased",
	"Pilot":            "Unreleased",
}

func seriesStatus(status string) string {
	return statuses[status]
}

func movieMetadata(movie *gotmdb.MovieDetails) items.Metadata {
	premiere := date(movie.ReleaseDate)

	return items.Metadata{
		Name:                text(movie.Title),
		OriginalTitle:       text(movie.OriginalTitle),
		Overview:            text(movie.Overview),
		OfficialRating:      rating(movieCertification(movie)),
		CommunityRating:     score(movie.VoteAverage),
		PremiereDate:        premiere,
		ProductionYear:      year(premiere),
		Taglines:            list(movie.Tagline),
		ProductionLocations: countries(movie.ProductionCountries),
		ProviderIds:         providerIDs(movie.ID, movie.IMDbID),
	}
}

func seriesMetadata(series *gotmdb.TVDetails) items.Metadata {
	premiere := date(series.FirstAirDate)

	metadata := items.Metadata{
		Name:                text(series.Name),
		OriginalTitle:       text(series.OriginalName),
		Overview:            text(series.Overview),
		Status:              text(seriesStatus(series.Status)),
		OfficialRating:      rating(seriesCertification(series)),
		CommunityRating:     score(series.VoteAverage),
		PremiereDate:        premiere,
		ProductionYear:      year(premiere),
		Taglines:            list(series.Tagline),
		ProductionLocations: countries(series.ProductionCountries),
		ProviderIds:         providerIDs(series.ID, seriesIMDbID(series)),
	}

	if !series.InProduction {
		metadata.EndDate = date(series.LastAirDate)
	}

	return metadata
}

func seasonMetadata(season *gotmdb.TVSeasonDetails) items.Metadata {
	premiere := date(season.AirDate)

	return items.Metadata{
		Name:            text(season.Name),
		Overview:        text(season.Overview),
		CommunityRating: score(season.VoteAverage),
		PremiereDate:    premiere,
		ProductionYear:  year(premiere),
		ProviderIds:     providerIDs(season.ID, ""),
	}
}

func episodeMetadata(episode *gotmdb.TVEpisodeDetails) items.Metadata {
	premiere := date(episode.AirDate)

	return items.Metadata{
		Name:            text(episode.Name),
		Overview:        text(episode.Overview),
		CommunityRating: score(episode.VoteAverage),
		PremiereDate:    premiere,
		ProductionYear:  year(premiere),
		ProviderIds:     providerIDs(episode.ID, episodeIMDbID(episode)),
	}
}

func movieCertification(movie *gotmdb.MovieDetails) string {
	if movie.MovieReleaseDatesAppend == nil || movie.ReleaseDates == nil {
		return ""
	}
	if movie.ReleaseDates.MovieReleaseDatesResults == nil {
		return ""
	}

	for _, released := range movie.ReleaseDates.Results {
		if released.Iso3166_1 != ratingCountry {
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

func seriesCertification(series *gotmdb.TVDetails) string {
	if series.TVContentRatingsAppend == nil || series.ContentRatings == nil {
		return ""
	}
	if series.ContentRatings.TVContentRatingsResults == nil {
		return ""
	}

	for _, rated := range series.ContentRatings.Results {
		if rated.Iso3166_1 == ratingCountry && rated.Rating != "" {
			return rated.Rating
		}
	}

	return ""
}

func seriesIMDbID(series *gotmdb.TVDetails) string {
	if series.TVExternalIDsAppend == nil || series.TVExternalIDs == nil {
		return ""
	}

	return series.IMDbID
}

func episodeIMDbID(episode *gotmdb.TVEpisodeDetails) string {
	if episode.TVEpisodeExternalIDsAppend == nil || episode.ExternalIDs == nil {
		return ""
	}

	return episode.ExternalIDs.IMDbID
}

func providerIDs(tmdbID int64, imdbID string) *map[string]string {
	ids := map[string]string{providerTmdb: strconv.FormatInt(tmdbID, 10)}
	if imdbID != "" {
		ids[providerImdb] = imdbID
	}

	return &ids
}

func countries(named []gotmdb.ProductionCountry) *[]string {
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

func score(value float32) *float64 {
	if value == 0 {
		return nil
	}
	rated := float64(value)

	return &rated
}
