package sonarr

import "github.com/FreekingDean/gojellyfin/internal/sources/arr"

const (
	SeriesPath   = "api/v3/series"
	EpisodesPath = "api/v3/episode"
)

type Series struct {
	ID     int    `json:"id"`
	TmdbID int    `json:"tmdbId"`
	Title  string `json:"title"`
	Year   int32  `json:"year"`
	Path   string `json:"path"`
	Tags   []int  `json:"tags"`
}

type Episode struct {
	SeasonNumber  int32    `json:"seasonNumber"`
	EpisodeNumber int32    `json:"episodeNumber"`
	Title         string   `json:"title"`
	HasFile       bool     `json:"hasFile"`
	File          arr.File `json:"episodeFile"`
}
